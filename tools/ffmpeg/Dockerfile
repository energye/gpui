FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
# gpui 双版本单文件构建镜像（本机 docker 和 GitHub Actions 共用）。
# 基础版零外库；高级版烧字四件 + openh264 全静态编进单文件（-fPIC no-shared），
# 用户机器不用装，许可保持 LGPL 2.1（x264/x265/fdk 一个不装）。
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential nasm pkg-config perl git curl \
      mingw-w64 \
      gcc-aarch64-linux-gnu libc6-dev-arm64-cross \
      gcc-arm-linux-gnueabihf libc6-dev-armhf-cross \
      gcc-multilib libc6-dev-i386 \
      libxml2-dev libva-dev libvdpau-dev libdrm-dev \
      meson ninja-build \
    && rm -rf /var/lib/apt/lists/*

# 版本钉死（和 recipe 注释同步，升级改这里 + 真源一行）。
ENV OPENSSL_VER=3.0.16 \
    ZLIB_VER=1.3.1 \
    FREETYPE_VER=2.13.2 \
    HARFBUZZ_VER=8.3.0 \
    FRIBIDI_VER=1.0.13 \
    FONTCONFIG_VER=2.15.0 \
    LIBASS_VER=0.17.1 \
    OPENH264_VER=2.4.1

# 源码由 workflow/本地脚本下载到 /tmp/deps（本镜像只管编，不管下）。
COPY build-deps.sh /build-deps.sh
RUN chmod +x /build-deps.sh
COPY build-one.sh /build-one.sh
RUN chmod +x /build-one.sh
