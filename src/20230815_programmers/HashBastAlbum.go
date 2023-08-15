package main

import (
	"fmt"
	"sort"
)

type Song struct {
	Index     int    `json:"index"`
	PlayCount int    `json:"play_count"`
	Genre     string `json:"genre"`
}

type TotalPlayList struct {
	Genre      string `json:"genre"`
	TotalCount int    `json:"total_count"`
}

func main() {
	genres := []string{"classic", "pop", "classic", "classic", "pop"}
	plays := []int{500, 600, 150, 800, 2500}

	result := solution(genres, plays)

	fmt.Print(result)
}

func solution(genres []string, plays []int) []int {
	maxLength := 2

	genreTotalPlays := make(map[string]TotalPlayList)
	songList := make([]Song, len(genres))
	sortedGenres := make([]TotalPlayList, 0, len(genres))

	for i, genre := range genres {
		play := plays[i]

		item, found := genreTotalPlays[genre]
		if found {
			item.TotalCount += play
			genreTotalPlays[genre] = item
		} else {
			genreTotalPlays[genre] = TotalPlayList{
				Genre:      genre,
				TotalCount: play,
			}
		}

		songList[i] = Song{
			Index:     i,
			PlayCount: play,
			Genre:     genre,
		}
	}

	for _, item := range genreTotalPlays {
		sortedGenres = append(sortedGenres, item)
	}

	sort.SliceStable(sortedGenres, func(i, j int) bool {
		return sortedGenres[i].TotalCount > sortedGenres[j].TotalCount
	})

	result := make([]int, 0, len(genres)*maxLength)
	for _, genre := range sortedGenres {
		songsInGenre := selectTopSongs(songList, genre.Genre, maxLength)
		result = append(result, songsInGenre...)
	}

	return result
}

func selectTopSongs(songs []Song, genre string, maxLength int) []int {
	songsInGenre := make([]Song, 0)

	for _, song := range songs {
		if song.Genre == genre {
			songsInGenre = append(songsInGenre, song)
		}
	}

	sort.SliceStable(songsInGenre, func(i, j int) bool {
		if songsInGenre[i].PlayCount == songsInGenre[j].PlayCount {
			return songsInGenre[i].Index < songsInGenre[j].Index
		}
		return songsInGenre[i].PlayCount > songsInGenre[j].PlayCount
	})

	selectedIndices := make([]int, 0, maxLength)
	for i := 0; i < len(songsInGenre) && i < maxLength; i++ { // 배열이 1개인 상황도 있음
		selectedIndices = append(selectedIndices, songsInGenre[i].Index)
	}

	return selectedIndices
}
