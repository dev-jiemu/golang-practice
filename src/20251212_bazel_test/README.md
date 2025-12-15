### CGO import -> Bazel 전환 테스트용 코드
#### Ref. VAD Filter in Golang

---

#### 기존 방식의 문제점
```dockerfile
# 로컬에서 디버깅 포함한 프로그램 실행흘 하려면 수동으로 ONNX Runtime 설치 필요
RUN wget https://github.com/microsoft/onnxruntime/.../onnxruntime-linux-x64-1.18.1.tgz
RUN tar -xzf onnxruntime-linux-x64-1.18.1.tgz
ENV CGO_CFLAGS="-I/usr/local/onnxruntime/include"
ENV CGO_LDFLAGS="-L/usr/local/onnxruntime/lib -lonnxruntime"
# ... 더 많은 환경변수 설정
```

이에따라 bazel + gazelle 활용으로 다른 환경에서도 설정 가능한지에 대한 테스트

#### 개선된 Bazel 방식
```bash
bazel build //:vad_filter_test  # ONNX Runtime 자동 다운로드 및 빌드
```

---

#### 1. Bazel 설치

```bash
# macOS
brew install bazel

# Linux (Ubuntu/Debian)
# https://bazel.build/install 참고
```

#### 2. 빌드 및 실행

```bash
# 자동으로 플랫폼 감지 및 ONNX Runtime 다운로드
bazel build //:vad_filter_test

# 실행
bazel run //:vad_filter_test

# 또는 Makefile 사용
make build
make run
```

---

### 🔧 플랫폼별 설정

#### 자동 감지 (권장)

Bazel이 자동으로 OS를 감지하여 적절한 ONNX Runtime 다운로드:

- **macOS**: `onnxruntime-osx-arm64-1.18.1.tgz` 자동 사용
- **Linux**: `onnxruntime-linux-x64-1.18.1.tgz` 자동 사용

```bash
# macOS에서
bazel build //:vad_filter_test  # ARM64 자동 선택

# Linux에서
bazel build //:vad_filter_test  # x64 자동 선택
```

#### 수동 지정 (선택적)

특정 플랫폼을 강제로 지정:

```bash
# macOS ARM64용 빌드
bazel build --config=darwin_arm64 //:vad_filter_test

# macOS x64용 빌드
bazel build --config=darwin_x64 //:vad_filter_test

# Linux x64용 빌드
bazel build --config=linux_x64 //:vad_filter_test

# Linux ARM64용 빌드
bazel build --config=linux_arm64 //:vad_filter_test
```

#### 크로스 플랫폼 빌드

```bash
# macOS에서 Linux용 빌드
bazel build --platforms=@rules_go//go/toolchain:linux_amd64 //:vad_filter_test
```


---

#### 🔍 작동 원리

1. **WORKSPACE.bazel**: ONNX Runtime을 GitHub에서 다운로드
2. **.bazelrc**: 현재 OS를 감지하여 적절한 config 적용
3. **BUILD.bazel**: 플랫폼별 라이브러리 자동 선택
4. **자동 링크**: CGO가 ONNX Runtime과 자동으로 연결

---

#### 빌드 캐시 문제

```bash
bazel clean --expunge
bazel build //:vad_filter_test
```

#### 상세 로그 확인

```bash
bazel build //:vad_filter_test --verbose_failures --subcommands
```

---


#### CGO 설정
```
# ONNX Runtime
alias(
    name = "onnxruntime_runtime_libs",
    actual = select({
        ":darwin_arm64": "@onnxruntime_darwin_arm64//:runtime_libs",
        ":linux_x64": "@onnxruntime_linux_x64//:runtime_libs",
        "//conditions:default": "@onnxruntime_darwin_arm64//:runtime_libs",
    }),
)

go_library(
    name = "vad_filter_lib",
    srcs = ["main.go", "cgo_darwin.go", "cgo_linux.go"],
    importpath = "github.com/example/vad-filter-test",
    cgo = True,
    cdeps = [":onnxruntime_lib"],  # Bazel이 자동으로 링크
    deps = ["@com_github_streamer45_silero_vad_go//speech"],
)

go_binary(
    name = "vad_filter_test",
    embed = [":vad_filter_lib"],
    data = [":onnxruntime_runtime_libs"],  # alias 사용으로 간소화
)
```
```
// rule 지정
load("@rules_go//go:def.bzl", "go_binary", "go_library")
// gazelle 사용
load("@gazelle//:def.bzl", "gazelle")

// 기본 프로젝트 모듈 지정 (이 프로젝트의 import path 루트를 지정)
# gazelle:prefix github.com/example/vad-filter-test
// 외부 라이브러리 모듈 지정
# gazelle:resolve go github.com/streamer45/silero-vad-go/speech @com_github_streamer45_silero_vad_go//speech
gazelle(name = "gazelle")
```


#### 📝 다음에 할일
- bazel 로 빌드한 결과물을 docker image 로 만든다면?