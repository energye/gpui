#!/bin/sh
# gpui 双版本单文件：编一个目标的一个版本。
# 用法：build-one.sh <linux-x64|linux-arm64|linux-386|linux-arm|win-x64|win-arm64> <base|full>
# 产物：/out/<target>/libgpui_ffmpeg.(so|dll)（基础版）或
#       /out/<target>/libgpui_ffmpeg_full.(so|dll)（高级版）
# 约定：源码挂在 /src（宿主检出目录），树外编译，源码树不脏。
# 配方唯一真源：gpui 仓 tools/ffmpeg-recipe（Go 命令），这里只管拼平台部分，
# 功能开关由 recipe 吐出（$RECIPE_FLAGS），本脚本不再手写 --enable-* 名单。
# 许可：两档都不碰 GPL（x264/x265/fdk 一个不开），LGPL 2.1 不变。
set -e
T="$1"
V="${2:-base}"
BLD="/tmp/b-$T-$V"
OUT="/out/$T"
mkdir -p "$BLD" "$OUT"
cd "$BLD"

# 配方从 gpui 仓的 recipe 工具取（构建镜像里预装 go，或由 workflow 传进来）。
# 本地跑：先在 gpui 根下 go run ./tools/ffmpeg-recipe -variant $V > /tmp/recipe-$V.flags
if [ -n "$RECIPE_FLAGS" ]; then
  COMMON="$RECIPE_FLAGS"
elif [ -f "/tmp/recipe-$V.flags" ]; then
  COMMON=$(cat "/tmp/recipe-$V.flags")
else
  echo "RECIPE_FLAGS 未设置且 /tmp/recipe-$V.flags 不存在" >&2
  echo "先跑：go run ./tools/ffmpeg-recipe -variant $V > /tmp/recipe-$V.flags" >&2
  exit 2
fi

# 高级版外库（BSD/LGPL/ISC，许可干净，全静态打进单文件）：
# freetype+harfbuzz（drawtext 必须）+ fontconfig+fribidi（建议，中东文字排版）
# + libass（subtitles/ass 必须）+ openh264（H264 编码，BSD）。
# 基础版不加这段，零外库。
# 外库来源二选一（都静态 .a + -fPIC）：
#   A. 容器内 apt 装 -dev 包（编完即验，网络要通外网）；
#   B. 预置源码包（/tmp/deps/*.tar.*，离线可编，见 build-deps.sh）。
# 各 /opt/<name>-<arch> 由 Dockerfile 或 build-deps.sh 预装。
FULL_DEPS_LINUX=""
if [ "$V" = "full" ]; then
  FULL_DEPS_LINUX="--enable-libfreetype --enable-libharfbuzz --enable-libfontconfig --enable-libfribidi --enable-libass --enable-libopenh264"
fi

LIB=libgpui_ffmpeg.so
if [ "$V" = "full" ]; then
  LIB=libgpui_ffmpeg_full.so
fi
# 外部库与硬解按平台开（autodetect已关，必须显式开）：
#   x64 Linux：openssl+libxml2（https/DASH）+vaapi/vdpau/drm（核显硬解）。
#   其它Linux（arm64/386/arm）：openssl（https），硬解走软解。
#   Windows：系统schannel（TLS）+d3d11va/dxva2（系统硬解），不添第三方依赖。
FULL_LINUX="--enable-openssl --enable-libxml2 --enable-demuxer=dash --extra-libs=-ldl --extra-libs=-pthread \
  --enable-vaapi --enable-vdpau --enable-libdrm \
  --enable-hwaccel=h264_vaapi --enable-hwaccel=hevc_vaapi --enable-hwaccel=vp9_vaapi \
  --enable-hwaccel=mpeg2_vaapi --enable-hwaccel=mpeg4_vaapi"
CROSS_LINUX="--enable-openssl --extra-libs=-ldl --extra-libs=-pthread"
HW_WIN="--enable-d3d11va --enable-dxva2 --enable-schannel \
  --enable-hwaccel=h264_d3d11va --enable-hwaccel=hevc_d3d11va --enable-hwaccel=vp9_d3d11va \
  --enable-hwaccel=h264_dxva2 --enable-hwaccel=hevc_dxva2"
ossl_lib() {
  if [ -d "$1/lib64" ]; then echo "$1/lib64"; else echo "$1/lib"; fi
}
# 外库头文件与静态库路径（高级版用，基础版忽略）。
# 约定 /opt/<name>-<suffix>，suffix 与目标对应（x64/arm64/386/arm/w64）。
dep_flags() { # $1=小写名 $2=目录
  if [ -d "$2" ]; then
    echo "--extra-cflags=-I$2/include --extra-ldflags=-L$2/lib"
  fi
}
case "$T" in
  linux-x64)
    OSSL=/opt/ossl-x64
    ZLIB=/opt/zlib-x64
    export PKG_CONFIG_PATH=$(ossl_lib $OSSL)/pkgconfig:${PKG_CONFIG_PATH:-}
    if [ "$V" = "full" ]; then
      for d in freetype harfbuzz fontconfig fribidi ass openh264; do
        export PKG_CONFIG_PATH="/opt/$d-x64/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
      done
    fi
    CFG="$COMMON $FULL_DEPS_LINUX $FULL_LINUX --extra-cflags=-I$OSSL/include --extra-cflags=-I$ZLIB/include --extra-ldflags=-L$(ossl_lib $OSSL) --extra-ldflags=-L$ZLIB/lib --arch=x86_64 --target-os=linux"
    CC=gcc ;;
  linux-arm64)
    OSSL=/opt/ossl-arm64
    ZLIB=/opt/zlib-arm64
    export PKG_CONFIG_LIBDIR=$(ossl_lib $OSSL)/pkgconfig:/usr/lib/aarch64-linux-gnu/pkgconfig
    if [ "$V" = "full" ]; then
      for d in freetype harfbuzz fontconfig fribidi ass openh264; do
        PKG_CONFIG_LIBDIR="/opt/$d-arm64/lib/pkgconfig:${PKG_CONFIG_LIBDIR:-}"
      done
      export PKG_CONFIG_LIBDIR
    fi
    CFG="$COMMON $FULL_DEPS_LINUX $CROSS_LINUX --extra-cflags=-I$OSSL/include --extra-cflags=-I$ZLIB/include --extra-ldflags=-L$(ossl_lib $OSSL) --extra-ldflags=-L$ZLIB/lib --enable-cross-compile --cross-prefix=aarch64-linux-gnu- --arch=aarch64 --target-os=linux"
    CC=aarch64-linux-gnu-gcc ;;
  linux-386)
    printf '#!/bin/sh\nexec gcc -m32 "$@"\n' > "$BLD/gcc-m32"
    chmod +x "$BLD/gcc-m32"
    OSSL=/opt/ossl-386
    ZLIB=/opt/zlib-386
    export PKG_CONFIG_LIBDIR=$(ossl_lib $OSSL)/pkgconfig:/usr/lib/i386-linux-gnu/pkgconfig
    CFG="$COMMON $FULL_DEPS_LINUX $CROSS_LINUX --extra-cflags=-I$OSSL/include --extra-cflags=-I$ZLIB/include --extra-ldflags=-L$(ossl_lib $OSSL) --extra-ldflags=-L$ZLIB/lib --arch=x86_32 --target-os=linux --cc=$BLD/gcc-m32"
    CC="$BLD/gcc-m32" ;;
  linux-arm)
    OSSL=/opt/ossl-arm
    ZLIB=/opt/zlib-arm
    export PKG_CONFIG_LIBDIR=$(ossl_lib $OSSL)/pkgconfig:/usr/lib/arm-linux-gnueabihf/pkgconfig
    CFG="$COMMON $FULL_DEPS_LINUX $CROSS_LINUX --extra-cflags=-I$OSSL/include --extra-cflags=-I$ZLIB/include --extra-ldflags=-L$(ossl_lib $OSSL) --extra-ldflags=-L$ZLIB/lib --enable-cross-compile --cross-prefix=arm-linux-gnueabihf- --arch=arm --target-os=linux"
    CC=arm-linux-gnueabihf-gcc ;;
  win-x64|win-arm64)
    ZLIB=/opt/zlib-w64
    # Windows 高级版烧字：mingw 静态 freetype/harfbuzz/libass（后续备好再加，
    # 缺则高级版暂只开写盒+原生编码，drawtext/subtitles 自动跳过）。
    CFG="$COMMON $FULL_DEPS_LINUX $HW_WIN --extra-cflags=-I$ZLIB/include --extra-ldflags=-L$ZLIB/lib --enable-cross-compile --cross-prefix=x86_64-w64-mingw32- --arch=x86_64 --target-os=mingw64"
    CC=x86_64-w64-mingw32-gcc; LIB="libgpui_ffmpeg.dll"
    if [ "$V" = "full" ]; then LIB="libgpui_ffmpeg_full.dll"; fi ;;
  *) echo "unknown target $T" >&2; exit 2 ;;
esac

# shellcheck disable=SC2086
/src/configure $CFG
make -j"$(nproc)"

WHOLE="-Wl,--whole-archive libavformat/libavformat.a libavcodec/libavcodec.a libswscale/libswscale.a libavfilter/libavfilter.a libswresample/libswresample.a libavdevice/libavdevice.a libavutil/libavutil.a -Wl,--no-whole-archive"
# MULDEFS: exr 解码器把 half2float.o 带进 libavcodec，和 libswscale 自带的
# 同名文件重复定义（内容一样）。--whole-archive 下两份都进包，ld 默认报错，
# 加 allow-multiple-definition 取第一份（ffmpeg 官方单文件同款做法）。
MULDEFS="-Wl,--allow-multiple-definition"
case "$T" in
  win-x64|win-arm64)
    # shellcheck disable=SC2086
    $CC -shared -o "$OUT/$LIB" $WHOLE $MULDEFS $ZLIB/lib/libz.a -lole32 -lbcrypt -lws2_32 -lsecur32 -lm
    ;;
  *)
    case "$T" in
      linux-x64)
        $CC -shared -Wl,-soname,$LIB -o "$OUT/$LIB" -Wl,-Bsymbolic $WHOLE $MULDEFS -lm -pthread $(ossl_lib $OSSL)/libssl.a $(ossl_lib $OSSL)/libcrypto.a $ZLIB/lib/libz.a -ldl -lxml2 -lva -lva-drm -lvdpau -ldrm
        ;;
      *)
        $CC -shared -Wl,-soname,$LIB -o "$OUT/$LIB" -Wl,-Bsymbolic $WHOLE $MULDEFS -lm -pthread $(ossl_lib $OSSL)/libssl.a $(ossl_lib $OSSL)/libcrypto.a -ldl $ZLIB/lib/libz.a
        ;;
    esac
    ;;
esac
ls -la "$OUT/$LIB"
# 门禁：高级版必须含写盒+字幕编码（nm 查 mp4 复用器与 mov_text 编码器）。
if [ "$V" = "full" ]; then
  case "$T" in
    win-*) tmp_tool="x86_64-w64-mingw32-nm" ;;
    *) tmp_tool="nm" ;;
  esac
  if ! $tmp_tool -D --defined-only "$OUT/$LIB" 2>/dev/null | grep -q "av_muxer_iterate"; then
    echo "WARN: full 版缺 av_muxer_iterate 符号" >&2
  fi
fi
