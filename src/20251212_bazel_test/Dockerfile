# Multi-stage build with Bazel
FROM --platform=linux/amd64 debian:bullseye-slim AS runtime

ENV APP_USER=appuser
ENV APP_UID=1001

RUN adduser --disabled-password --gecos '' --uid ${APP_UID} ${APP_USER} && \
    apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates wget xz-utils && \
    rm -rf /var/lib/apt/lists/*

# ffmpeg install
RUN wget -q https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz &&\
    tar -xJf ffmpeg-release-amd64-static.tar.xz &&\
    mv ./ffmpeg-7.0.2-amd64-static/ffmpeg /usr/bin/ffmpeg &&\
    rm -rf ffmpeg-release-amd64-static.tar.xz ffmpeg-7.0.2-amd64-static

# ONNX runtime library
RUN wget -q https://github.com/microsoft/onnxruntime/releases/download/v1.18.1/onnxruntime-linux-x64-1.18.1.tgz && \
    tar -xzf onnxruntime-linux-x64-1.18.1.tgz && \
    mv ./onnxruntime-linux-x64-1.18.1 /usr/local/onnxruntime && \
    rm -rf onnxruntime-linux-x64-1.18.1.tgz

ENV LD_LIBRARY_PATH="/usr/local/onnxruntime/lib"

WORKDIR /app

# Copy Bazel build artifact
# Before building this Dockerfile, run: bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //:vad_filter_test
COPY bazel-bin/vad_filter_test_/vad_filter_test /app/vad_filter_test

# Download silero VAD model
RUN wget https://github.com/snakers4/silero-vad/raw/master/src/silero_vad/data/silero_vad.onnx -O /app/silero_vad.onnx

RUN chown -R ${APP_USER}:${APP_USER} /app

USER ${APP_USER}

ENTRYPOINT ["/app/vad_filter_test"]