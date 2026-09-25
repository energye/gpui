#!/bin/sh
# 双版本外库静态编译（在构建镜像里跑一遍，产物 /opt/<name>-<arch>）。
# 用法：build-deps.sh [x64|arm64|386|arm|w64|all]
# 全静态（no-shared + -fPIC），只产 .a，不欠动态账。
# 许可：freetype(FTL/GPL二选一，取FTL)+harfbuzz(MIT)+fontconfig(MIT)+
# fribidi(LGPL)+libass(ISC)+openh264(BSD)+openssl(Apache-2)+zlib(zlib)，
# 和 LGPL 2.1 拼盘没问题；x264/x265/fdk 不在此装。
set -e
WHICH="${1:-x64}"
SRC=/tmp/deps
mkdir -p "$SRC"
cd "$SRC"

# 各库版本与 Dockerfile ENV 同源（改一处即可，另一处同步）。
FT=${FREETYPE_VER:-2.13.2}
HB=${HARFBUZZ_VER:-8.3.0}
FB=${FRIBIDI_VER:-1.0.13}
FC=${FONTCONFIG_VER:-2.15.0}
ASS=${LIBASS_VER:-0.17.1}
H264=${OPENH264_VER:-2.4.1}
OSSL=${OPENSSL_VER:-3.0.16}
ZL=${ZLIB_VER:-1.3.1}

need() { # file url
  if [ ! -f "$1" ]; then curl -sSL -o "$1" "$2"; fi
}
need freetype-$FT.tar.gz "https://download.savannah.gnu.org/releases/freetype/freetype-$FT.tar.gz"
need harfbuzz-$HB.tar.xz "https://github.com/harfbuzz/harfbuzz/releases/download/$HB/harfbuzz-$HB.tar.xz"
need fribidi-$FB.tar.xz "https://github.com/fribidi/fribidi/releases/download/v$FB/fribidi-$FB.tar.xz"
need fontconfig-$FC.tar.gz "https://www.freedesktop.org/software/fontconfig/release/fontconfig-$FC.tar.gz"
need libass-$ASS.tar.gz "https://github.com/libass/libass/releases/download/$ASS/libass-$ASS.tar.gz"
need openh264-$H264.tar.gz "https://github.com/cisco/openh264/archive/refs/tags/v$H264.tar.gz"
need openssl-$OSSL.tar.gz "https://www.openssl.org/source/openssl-$OSSL.tar.gz"
need zlib-$ZL.tar.gz "https://zlib.net/zlib-$ZL.tar.gz"

build_one_arch() { # arch-tag cc cxx
  TAG="$1"; CC="$2"; CXX="$3"
  PF=/opt
  # freetype（静态，harfbuzz 关，免循环依赖）。
  rm -rf ft-$TAG && mkdir ft-$TAG && tar -xzf freetype-$FT.tar.gz -C ft-$TAG --strip-components=1
  (cd ft-$TAG && CC="$CC" CFLAGS="-fPIC" ./configure --disable-shared --enable-static --without-harfbuzz --without-bzip2 --without-png --prefix=$PF/freetype-$TAG && make -j"$(nproc)" && make install)
  # harfbuzz（静态，指向上一步的 freetype）。
  rm -rf hb-$TAG && mkdir hb-$TAG && tar -xJf harfbuzz-$HB.tar.xz -C hb-$TAG --strip-components=1
  (cd hb-$TAG && CC="$CC" CXX="$CXX" CFLAGS="-fPIC" CXXFLAGS="-fPIC" PKG_CONFIG_PATH=$PF/freetype-$TAG/lib/pkgconfig meson setup --default-library=static --prefix=$PF/harfbuzz-$TAG build && ninja -C build && ninja -C build install)
  # fribidi + fontconfig + libass（静态）。
  rm -rf fb-$TAG && mkdir fb-$TAG && tar -xJf fribidi-$FB.tar.xz -C fb-$TAG --strip-components=1
  (cd fb-$TAG && CC="$CC" CFLAGS="-fPIC" meson setup --default-library=static --prefix=$PF/fribidi-$TAG build && ninja -C build && ninja -C build install)
  rm -rf fc-$TAG && mkdir fc-$TAG && tar -xzf fontconfig-$FC.tar.gz -C fc-$TAG --strip-components=1
  (cd fc-$TAG && CC="$CC" CFLAGS="-fPIC" PKG_CONFIG_PATH=$PF/freetype-$TAG/lib/pkgconfig:$PF/fribidi-$TAG/lib/pkgconfig ./configure --disable-shared --enable-static --disable-docs --prefix=$PF/fontconfig-$TAG && make -j"$(nproc)" && make install)
  rm -rf as-$TAG && mkdir as-$TAG && tar -xzf libass-$ASS.tar.gz -C as-$TAG --strip-components=1
  (cd as-$TAG && CC="$CC" CFLAGS="-fPIC" PKG_CONFIG_PATH=$PF/freetype-$TAG/lib/pkgconfig:$PF/harfbuzz-$TAG/lib/pkgconfig:$PF/fribidi-$TAG/lib/pkgconfig:$PF/fontconfig-$TAG/lib/pkgconfig ./configure --disable-shared --enable-static --disable-fontconfig --prefix=$PF/ass-$TAG && make -j"$(nproc)" && make install)
  # openh264（静态）。
  rm -rf h264-$TAG && mkdir h264-$TAG && tar -xzf openh264-$H264.tar.gz -C h264-$TAG --strip-components=1
  (cd h264-$TAG && make CC="$CC" CXX="$CXX" ARCH=$(arch_map "$TAG") USE_ASM=No -j"$(nproc)" && make PREFIX=$PF/openh264-$TAG install)
}

arch_map() {
  case "$1" in
    x64|w64) echo x86_64 ;; arm64) echo aarch64 ;; 386) echo i386 ;; arm) echo arm ;;
  esac
}

case "$WHICH" in
  x64) build_one_arch x64 gcc g++ ;;
  arm64) build_one_arch arm64 aarch64-linux-gnu-gcc aarch64-linux-gnu-g++ ;;
  386) build_one_arch 386 "gcc -m32" "g++ -m32" ;;
  arm) build_one_arch arm arm-linux-gnueabihf-gcc arm-linux-gnueabihf-g++ ;;
  w64) build_one_arch w64 x86_64-w64-mingw32-gcc x86_64-w64-mingw32-g++ ;;
  all) for a in x64 arm64 386 arm w64; do "$0" "$a"; done ;;
  *) echo "unknown $WHICH" >&2; exit 2 ;;
esac
echo "deps $WHICH done"
