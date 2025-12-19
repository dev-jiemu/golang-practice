package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/streamer45/silero-vad-go/speech"
)

type HallucinationSegment struct {
	StartIdx  int
	EndIdx    int
	StartTime float64
	EndTime   float64
	Text      string
}

type TranslatorWhisper struct {
	speechSegments []speech.Segment
}

func NewTranslatorWhisper(segments []speech.Segment) *TranslatorWhisper {
	return &TranslatorWhisper{
		speechSegments: segments,
	}
}

var Translator *TranslatorWhisper
var WhisperConfig *Config

// maxFileSize : webm 파일 용량체크 : 안전하게 MAX 12MB 로 지정
const (
	maxFileSize      = 25 * 1024 * 1024 // 25MB = 25MiB
	mb               = 1024 * 1024
	modelNameWhisper = "whisper-1"
)

// RequestTranscription : OpenAi Whisper API 로 자막 데이터 요청
func (v *TranslatorWhisper) RequestTranscription(ctx context.Context, job *Job) ([]SubtitleSegment, error) {
	var err error
	var subtitles []SubtitleSegment

	inputPath := job.OriginalAudioPath
	audioPath := job.AudioPath // 이후 temp 경로 필요

	// .webm 변환 (전체 파일에 대한 변환)
	if err = ExtractAudio(ctx, inputPath, audioPath); err != nil {
		return nil, fmt.Errorf("extract audio fail: %w", err)
	}

	// TODO : Chunk 로 분할 -> 병렬 고루틴 처리

	// whisper 호출
	response, err := v.CallWhisperApi(ctx, audioPath, job)
	if err != nil {
		return nil, err
	}

	// 환각체크 및 재시도
	improvedResponse, err := v.RetryHallucinationSegments(response, job)
	if err != nil {
		improvedResponse = response
	}

	// 자막생성
	subtitles = v.ConvertWhisperResponse(improvedResponse)

	return subtitles, err
}

// CallWhisperApi : Whisper Speech To Text API
// audioPath : 원본파일과 일부 재시도 하는 temp 파일 경로 둘다 해당됨
func (v *TranslatorWhisper) CallWhisperApi(ctx context.Context, audioPath string, job *Job) (*WhisperResponse, error) {
	var err error

	// 용량체크
	if _, err = v.checkWebmFileSize(audioPath); err != nil {
		return nil, fmt.Errorf("check audio fail: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// timestamp issue 로 인해 whisper 모델 강제됨
	writer.WriteField("model", modelNameWhisper)
	writer.WriteField("response_format", "verbose_json")
	writer.WriteField("language", "en")
	writer.WriteField("timestamp_granularities[]", "word")
	writer.WriteField("timestamp_granularities[]", "segment")
	writer.WriteField("temperature", "0") // 환각 줄이기: 결정론적 디코딩

	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("open audio file: %s", err)
	}
	defer file.Close()

	// temp
	filename := job.RId + ".webm"

	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %s", err)
	}
	if _, err := io.Copy(fileWriter, file); err != nil {
		return nil, fmt.Errorf("copy audio file: %s", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// 일단 5분 정도로 잡음
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", &body)
	if err != nil {
		return nil, fmt.Errorf("http request fail : %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", WhisperConfig.OpenAIKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP Request Fail : %v\n", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response read fail : %v\n", err)
	}

	// POST 는 206 응답을 주지 않음
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper api error: status=%s body=%s", resp.Status, string(responseBody))
	}

	ret := &WhisperResponse{}
	err = json.Unmarshal(responseBody, ret)
	if err != nil {
		return nil, fmt.Errorf("json parsing error : %v\n", err)
	}

	return ret, err
}

func (v *TranslatorWhisper) checkWebmFileSize(filepath string) (int64, error) {
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file %s does not exist", filepath)
		}
		return 0, fmt.Errorf("file info not found: %v", err)
	}

	fileSize := fileInfo.Size() // bytes

	if fileSize > maxFileSize {
		currentMB := float64(fileSize) / float64(mb) // 실제 MB
		maxMB := float64(maxFileSize) / float64(mb)  // 25.00

		return 0, fmt.Errorf(
			"file size too big :: current - %.2fMB, max - %.2fMB",
			currentMB,
			maxMB,
		)
	}

	return fileSize, nil
}

// RetryHallucinationSegments : 환각이 발생한 자막 재호출 시도
func (v *TranslatorWhisper) RetryHallucinationSegments(original *WhisperResponse, job *Job) (*WhisperResponse, error) {
	minRepetitions := 5
	hallucinationRetry := 1

	hallucinations := DetectAllHallucinations(original.Segments, minRepetitions, 0.5)
	if len(original.Segments) == 0 {
		return original, fmt.Errorf("empty segments")
	}

	if len(hallucinations) == 0 {
		return original, fmt.Errorf("no hallucination segments")
	}

	merged := original

	for i := 0; i < hallucinationRetry; i++ {
		// 인덱스가 꼬이는것 같아서 역순으로 처리하겠음
		for idx := len(hallucinations) - 1; idx >= 0; idx-- {
			halluc := hallucinations[idx]

			//reqLog.Debug("hallucination segment",
			//	"start_idx", halluc.StartIdx,
			//	"end_idx", halluc.EndIdx,
			//	"start_time", halluc.StartTime,
			//	"end_time", halluc.EndTime,
			//	"repeated_text", halluc.Text)

			var segmentResponse *WhisperResponse
			duration := halluc.EndTime - halluc.StartTime

			// 환각 데이터가 너무 짧으면 요청하지 않고 건너뜀 (이후 convert 단계에서도 자막 데이터 생성하지 않을 것)
			// 요청하면 400 bad request 발생함
			if duration < 0.1 {
				// reqLog.Debug("skip too short hallucination", "duration", duration, "start_idx", halluc.StartIdx, "end_idx", halluc.EndIdx)

				// 해당 경우는 단일 세그먼트가 환각을 일으켰는데, 자막 데이터가 너무 짧아서 재요청 할 수 없는 상황임
				// 이후 convert 하는 과정에서 text 가 빈 값이면 제외되는 로직이 있으므로 빈 객체 만들어서 자동 merged 처리
				segmentResponse = &WhisperResponse{
					Task:     merged.Task,
					Language: merged.Language,
					Duration: duration,
					Segments: []WhisperSegment{
						{
							ID:    0,
							Start: 0,
							End:   duration,
							Text:  "",
						},
					},
					Words: []WhisperWord{},
				}
			} else {
				ctx := context.Background() // TODO : need fix

				// TODO : FIX
				hallucInput := ""
				hallucOutput := ""
				err := ExtractAudioForWhisperChunk(ctx, hallucInput, hallucOutput, halluc.StartTime, halluc.EndTime)
				if err != nil {
					fmt.Println("audio extractor fail", "err", err)
					continue
				}

				segmentResponse, err = v.CallWhisperApi(ctx, hallucOutput, job)
				if err != nil {
					fmt.Println("whisper api fail", "err", err)
					continue
				}
			}

			if segmentResponse != nil && len(segmentResponse.Segments) > 0 { // 정상 응답이든 빈 응답이든 merge
				merged = v.mergeSegments(merged, segmentResponse, halluc)
			}
		}

		// 다음 시도를 위해 환각 재감지 (없으면 중지)
		hallucinations = DetectAllHallucinations(merged.Segments, minRepetitions, 0.5)
		fmt.Println("Re-checking hallucinations on merged segments", "count", len(hallucinations))
		if len(hallucinations) == 0 {
			break
		}
	}

	return merged, nil
}

func (v *TranslatorWhisper) mergeSegments(original *WhisperResponse, retried *WhisperResponse, halluc HallucinationSegment) *WhisperResponse {
	ret := &WhisperResponse{
		Task:     original.Task,
		Language: original.Language,
		Duration: original.Duration,
		Words:    []WhisperWord{},
		Segments: []WhisperSegment{},
		// Text 어차피 안쓰니까 비워둠!
		// Text: original.Text
	}

	timeOffset := halluc.StartTime

	adjustedSegments := make([]WhisperSegment, len(retried.Segments))
	for idx, seg := range retried.Segments {
		adjustedSegments[idx] = seg
		adjustedSegments[idx].Start += timeOffset
		adjustedSegments[idx].End += timeOffset
		adjustedSegments[idx].ID = halluc.StartIdx + idx
	}

	adjustedWords := make([]WhisperWord, len(retried.Words))
	for idx, word := range retried.Words {
		adjustedWords[idx] = word
		adjustedWords[idx].Start += timeOffset
		adjustedWords[idx].End += timeOffset
	}

	// original segment 와 adjust segment 를 결합
	ret.Segments = append(ret.Segments, original.Segments[:halluc.StartIdx]...)
	ret.Segments = append(ret.Segments, adjustedSegments...)

	afterSegments := original.Segments[halluc.EndIdx:]
	idOffset := len(ret.Segments)

	for i, seg := range afterSegments {
		seg.ID = idOffset + i
		ret.Segments = append(ret.Segments, seg)
	}

	wordStartIdx := 0
	wordEndIdx := len(original.Words)

	for i, word := range original.Words {
		if word.Start >= halluc.StartTime {
			wordStartIdx = i
			break
		}
	}

	for i := len(original.Words) - 1; i >= 0; i-- {
		if original.Words[i].End <= halluc.EndTime {
			wordEndIdx = i + 1
			break
		}
	}

	// 안전하게 범위 체크
	if wordStartIdx > len(original.Words) {
		wordStartIdx = len(original.Words)
	}
	if wordEndIdx > len(original.Words) {
		wordEndIdx = len(original.Words)
	}

	ret.Words = append(ret.Words, original.Words[:wordStartIdx]...)
	ret.Words = append(ret.Words, adjustedWords...)
	ret.Words = append(ret.Words, original.Words[wordEndIdx:]...)

	return ret
}

// 자막 데이터 필터링용
var spaceRe = regexp.MustCompile(`\s+`)

// ConvertWhisperResponse : OpenAI 응답 데이터를 json, srt 파일로 만들기 위한 struct 생성 (json format)
func (v *TranslatorWhisper) ConvertWhisperResponse(response *WhisperResponse) []SubtitleSegment {
	convertedSegments := make([]SubtitleSegment, 0, len(response.Segments))

	wordTotalLength := len(response.Words)
	wordCursor := 0 // 과도한 탐색 방지를 위해 직전에 탐색한 인덱스 값 담아두는 용도

	const tolerance = 0.05

	for _, seg := range response.Segments {

		// Text 가 없으면 건너뜀
		if len(seg.Text) == 0 {
			continue
		}

		// subtitle object create
		subtitle := SubtitleSegment{
			Idx:                     seg.ID,
			StartTime:               roundSeconds(seg.Start),
			EndTime:                 roundSeconds(seg.End),
			Sentence:                normalizeWhitespace(seg.Text),
			SentenceConfidenceScore: seg.AvgLogProb,
			NoSpeechProb:            seg.NoSpeechProb,
			CompressionRatio:        seg.CompressionRatio,
		}

		// words array index search
		startIdx := wordCursor
		for startIdx < wordTotalLength && (response.Words[startIdx].End+tolerance) < seg.Start {
			startIdx++
		}

		endIdx := startIdx
		for endIdx < wordTotalLength && (response.Words[endIdx].Start-tolerance) <= seg.End {
			endIdx++
		}

		// [startIdx, endIdx)
		frames := make([]SentenceFrames, 0, endIdx-startIdx)
		frameIdx := 0

		for k := startIdx; k < endIdx; k++ {
			word := response.Words[k]

			// 혹시모르니 가드
			if (word.End+tolerance) < seg.Start || (word.Start-tolerance) > seg.End {
				continue
			}

			wordStart := word.Start
			wordEnd := word.End

			if wordStart < subtitle.StartTime {
				wordStart = subtitle.StartTime
			}
			if wordEnd > subtitle.EndTime {
				wordEnd = subtitle.EndTime // segment 끝 시간으로 자름 (오버타임이슈 방지)
			}

			frames = append(frames, SentenceFrames{
				WordIdx:       frameIdx,
				Word:          normalizeWhitespace(word.Word),
				WordStartTime: roundSeconds(wordStart),
				WordEndTime:   roundSeconds(wordEnd),
			})
			frameIdx++
		}

		subtitle.SentenceFrames = frames
		convertedSegments = append(convertedSegments, subtitle)

		// next search index update
		wordCursor = endIdx
	}

	return convertedSegments
}

// DetectAllHallucinations : 환각 범위 체크 고도화 Ver. (N-gram check)
func DetectAllHallucinations(segments []WhisperSegment, minRepetitions int, minRepetitionRatio float64) []HallucinationSegment {
	hallucinations := make([]HallucinationSegment, 0)

	// 1. 세그먼트 간 반복 체크
	crossSegmentHalls := DetectHallucinations(segments, minRepetitions)
	hallucinations = append(hallucinations, crossSegmentHalls...)

	// 2. 단위 세그먼트 내 반복체크
	inSegmentHalls := DetectInSegmentHallucinations(segments, minRepetitionRatio)

	// 1 + 2
	existingRanges := make(map[int]bool)
	for _, h := range hallucinations {
		for i := h.StartIdx; i < h.EndIdx; i++ {
			existingRanges[i] = true
		}
	}

	for _, h := range inSegmentHalls {
		if !existingRanges[h.StartIdx] {
			hallucinations = append(hallucinations, h)
		}
	}

	// 오름차순으로 정렬 (chunk 재요청 할때 역순으로 접근하기 때문에 인덱스 꼬임 방지)
	sort.Slice(hallucinations, func(i, j int) bool {
		return hallucinations[i].StartIdx < hallucinations[j].StartIdx
	})

	// duration 0.1 이하의 반복 데이터면 삭제(0.1 미만은 단일 데이터일 확률이 높음)
	// 데이터 자체를 제거는 하지 않고 이대로 convert 영역까지 들고가서 거기서 걸러냄
	for _, halluc := range hallucinations {
		duration := halluc.EndTime - halluc.StartTime

		if duration < 0.1 {
			halluc.Text = ""
		}
	}

	return hallucinations
}

// DetectHallucinations : 환각 범위 체크
func DetectHallucinations(segments []WhisperSegment, minRepetitions int) []HallucinationSegment {
	hallucinations := make([]HallucinationSegment, 0)
	i := 0

	for i < len(segments) {
		text := normalizeWhitespace(segments[i].Text)

		if len(text) == 0 {
			i++
			continue
		}

		startIdx := i
		lastValidIdx := i
		j := i + 1
		consecutiveCount := 1

		for j < len(segments) {
			nextText := normalizeWhitespace(segments[j].Text)
			if len(nextText) == 0 {
				j++
				continue
			}

			if !strings.EqualFold(text, nextText) {
				break
			}

			consecutiveCount++
			lastValidIdx = j
			j++
		}

		if consecutiveCount >= minRepetitions {
			hallucination := HallucinationSegment{
				StartIdx:  startIdx,
				EndIdx:    lastValidIdx + 1,
				StartTime: segments[startIdx].Start,
				EndTime:   segments[lastValidIdx].End,
				Text:      text,
			}
			hallucinations = append(hallucinations, hallucination)
			i = j
		} else {
			i++
		}

	}

	return hallucinations
}

// DetectInSegmentHallucinations : 단일 세그먼트 내 단어 반복 체크
func DetectInSegmentHallucinations(segments []WhisperSegment, minRepetitionRatio float64) []HallucinationSegment {
	hallucinations := make([]HallucinationSegment, 0)

	for i, seg := range segments {
		text := normalizeWhitespace(seg.Text)
		if len(text) < 20 { // 너무 짧은건 체크 안함
			continue
		}

		if hasRepetitivePattern(text, minRepetitionRatio) {
			hallucination := HallucinationSegment{
				StartIdx:  i,
				EndIdx:    i + 1,
				StartTime: seg.Start,
				EndTime:   seg.End,
				Text:      text,
			}
			hallucinations = append(hallucinations, hallucination)
		}
	}

	return hallucinations
}

// hasRepetitivePattern : 세그먼트 내 반복패턴 확인
func hasRepetitivePattern(text string, minRatio float64) bool {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 10 {
		return false
	}

	wordCount := make(map[string]int, len(words))
	for _, word := range words {
		if len(word) > 2 {
			wordCount[word]++
		}
	}

	maxCount := 0
	for _, count := range wordCount {
		if count > maxCount {
			maxCount = count
		}
	}

	ratio := float64(maxCount) / float64(len(words))
	if ratio >= minRatio {
		return true
	}

	if hasRepeatingNgrams(words, 2, 5, minRatio) {
		return true
	}

	return false
}

// hasRepeatingNgrams : N-gram 반복패턴 체크
func hasRepeatingNgrams(words []string, ngramSize, minRepeat int, minRatio float64) bool {
	if len(words) < ngramSize*minRepeat {
		return false
	}

	ngramCount := make(map[string]int)

	for i := 0; i <= len(words)-ngramSize; i++ {
		ngram := strings.Join(words[i:i+ngramSize], " ")
		ngramCount[ngram]++
	}

	totalNgrams := len(words) - ngramSize + 1
	for _, count := range ngramCount {
		if count >= minRepeat {
			ratio := float64(count) / float64(totalNgrams)
			if ratio >= minRatio {
				return true
			}
		}
	}

	return false
}
