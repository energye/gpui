// Package aac implements the A1 audio front end (AAC-LC first).
//
// Scope (A1 landing 2): MP4 audio Raw packets are already framed by the
// shell (one sample = one raw_data_block); this package owns the config
// (AudioSpecificConfig), the bare-stream framing (ADTS split), the full
// spectral decode (ICS + Huffman + dequant + M/S + intensity + TNS +
// IMDCT + windowing) and the PCM carrier. PCE channel configs and
// 960-line frames stay honest ErrUnsupported; nothing here fakes PCM.
//
// ffmpeg peers (read-only,上级 gogpu/ffmpeg, no code copied):
//
//	libavcodec/mpeg4audio.c: ff_mpeg4audio_get_config_gb (ASC base +
//	  SBR/PS extension) and ff_mpeg4audio_channels (chan config map).
//	libavcodec/aac/aacdec.c: decode_audio_specific_config(_gb) (ASC
//	  entry, lines ~1119-1220), ff_aac_decode_init (extradata path),
//	  parse_adts_frame_header + aac_decode_frame_int (ADTS syncword
//	  0xFFF gate, lines ~2505-2560).
//	libavformat/mov_esds.c: ff_mov_read_esds (ES->DecConfig->DecSpecific
//	  descriptor walk, tag 0x03/0x04/0x05); libavformat/mov.c:
//	  mov_read_audio (mp4a entry channels/rate, version 0/1/2).
package aac
