package main

import "sort"

type WhisperSegment struct {
	Id    int
	Start float64
	End   float64
	Text  string
}

type SpeechSegment struct {
	SpeechStartAt float64
	SpeechEndAt   float64
}

func getTestData() ([]SpeechSegment, []WhisperSegment) {
	left := []SpeechSegment{
		{19.330000, 19.614000},
		{22.402000, 23.486000},
		{23.778000, 23.998000},
		{24.098000, 24.318000},
		{32.354000, 33.118000},
		{33.314000, 33.438000},
		{35.074, 35.742},
		{36.450, 37.886},
	}

	right := []WhisperSegment{
		{0, 31.360000610351562, 32.5, " Hello, Merry Christmas."},
		{1, 32.619998931884766, 33.119998931884766, " Merry Christmas."},
		{2, 33.5, 33.63999938964844, " Merrick."},
		{3, 34.2599983215332, 34.52000045776367, " Hey."},
		{4, 34.91999816894531, 37.060001373291016, " You know the gifts that you get on Christmas, right?"},
		{5, 37.2599983215332, 37.68000030517578, " What?"},
		{6, 39.08000183105469, 41.41999816894531, " Merry Christmas, Christmas Eve, Eve, Eve."},
	}

	return left, right
}

func SortSpeechSegments(segments []SpeechSegment) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].SpeechStartAt < segments[j].SpeechStartAt
	})
}

func SortWhisperSegments(segments []WhisperSegment) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Start < segments[j].Start
	})
}
