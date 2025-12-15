# onnxruntime.BUILD
# ONNX Runtime을 Bazel에서 사용할 수 있도록 하는 BUILD 파일

package(default_visibility = ["//visibility:public"])

cc_library(
    name = "onnxruntime",
    hdrs = glob(
        [
            "include/**/*.h",
            "include/**/*.hpp",
        ],
        allow_empty = True,
    ),
    includes = ["include"],
    linkopts = select({
        "@platforms//os:osx": [
            "-Wl,-rpath,@loader_path/../lib",
        ],
        "@platforms//os:linux": [
            "-Wl,-rpath,$$ORIGIN/../lib",
        ],
        "//conditions:default": [],
    }),
    srcs = select({
        "@platforms//os:osx": glob(["lib/libonnxruntime*.dylib"], allow_empty = True),
        "@platforms//os:linux": glob(["lib/libonnxruntime*.so*"], allow_empty = True),
        "//conditions:default": [],
    }),
)

# 런타임에 필요한 동적 라이브러리 파일들
filegroup(
    name = "runtime_libs",
    srcs = select({
        "@platforms//os:osx": glob(["lib/*.dylib"], allow_empty = True),
        "@platforms//os:linux": glob(["lib/*.so*"], allow_empty = True),
        "//conditions:default": [],
    }),
)
