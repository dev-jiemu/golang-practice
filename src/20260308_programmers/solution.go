package main

// 코딩테스트 전형 중 복기하고 싶은 문제 코드
// 그냥 딱 보자마자 생각난게 dfs 였고 trie 노드 구조 비슷하게 가면 될것 같다 생각해서 하긴 했는데....
// 개선사항 없으려나 싶어서 복기 겸 남김

import (
	"sort"
	"strings"
)

type Directory struct {
	next map[string]*Directory
}

func solution(directory []string, command []string) []string {
	// directory 만들기
	root := &Directory{}
	root.next = make(map[string]*Directory)

	// 하위 directory 설정
	for _, dir := range directory {
		if dir == "/" {
			continue
		}
		parts := parsePath(dir)
		cur := root
		for _, part := range parts {
			if cur.next[part] == nil {
				cur.next[part] = &Directory{}
				cur.next[part].next = make(map[string]*Directory)
			}
			cur = cur.next[part]
		}
	}

	// command sprit " "
	// mkdir, cp, rm 판단
	for _, cmd := range command {
		tokens := strings.Fields(cmd)

		switch tokens[0] {
		case "mkdir":
			parts := parsePath(tokens[1])
			current := root
			for _, part := range parts {
				if current.next[part] == nil {
					current.next[part] = &Directory{}
					current.next[part].next = make(map[string]*Directory)
				}
				current = current.next[part]
			}

		case "rm":
			parts := parsePath(tokens[1])
			if len(parts) == 0 {
				break // 없으면 진행 안함
			}
			parent := getNode(root, parts[:len(parts)-1])
			if parent != nil { // 하위까지 다 지움
				delete(parent.next, parts[len(parts)-1])
			}

		case "cp":
			srcPath := parsePath(tokens[1])
			dstPath := parsePath(tokens[2])
			srcNode := getNode(root, srcPath)
			if srcNode == nil {
				break
			}
			dstParent := getNode(root, dstPath)
			if dstParent == nil { // 없으면 진행 안함
				break
			}

			// dst 아래로 복사
			srcName := srcPath[len(srcPath)-1]
			dstParent.next[srcName] = copyNode(srcNode)
		}
	}

	var result []string
	collectPaths(root, "/", &result)

	return result
}

// 경로 파싱
func parsePath(path string) []string {
	var result []string
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// 노드 탐색
func getNode(root *Directory, parts []string) *Directory {
	current := root
	for _, part := range parts {
		if current.next[part] == nil {
			return nil
		}
		current = current.next[part]
	}

	return current // 찾은거
}

func copyNode(dir *Directory) *Directory {
	newDir := &Directory{}
	newDir.next = make(map[string]*Directory)
	for k, v := range dir.next {
		newDir.next[k] = copyNode(v)
	}

	return newDir
}

// collect : dfs 로 탐색
func collectPaths(dir *Directory, current string, result *[]string) {
	*result = append(*result, current)

	names := make([]string, 0, len(dir.next))
	for name := range dir.next {
		names = append(names, name)
	}

	// 사전 순으로 정렬
	sort.Strings(names)

	for _, name := range names {
		var nextPath string
		if current == "/" {
			nextPath = "/" + name
		} else {
			nextPath = current + "/" + name
		}
		collectPaths(dir.next[name], nextPath, result)
	}
}
