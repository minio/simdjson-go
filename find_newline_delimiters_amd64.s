//+build !noasm !appengine gc

// _find_newline_delimiters(raw []byte) (mask uint64)
TEXT ·_find_newline_delimiters(SB), 7, $0
	MOVQ    raw+0(FP), SI        // SI: &raw
	MOVQ    quoteMask+24(FP), DX // get quotemask
	VMOVDQU (SI), Y8             // load low 32-bytes
	VMOVDQU 0x20(SI), Y9         // load high 32-bytes

	CALL ·__find_newline_delimiters(SB)

	MOVQ BX, mask+32(FP) // store result
	VZEROUPPER
	RET

TEXT ·__find_newline_delimiters(SB), 7, $0
	MOVQ         $0x0a, BX // get newline
	MOVQ         BX, X11
	VPBROADCASTB X11, Y11

	VPCMPEQB  Y8, Y11, Y10
	VPCMPEQB  Y9, Y11, Y11
	VPMOVMSKB Y10, BX
	VPMOVMSKB Y11, CX
	SHLQ      $32, CX
	ORQ       CX, BX       // BX is resulting mask of newline chars
	ANDNQ     BX, DX, BX   // clear out newline delimiters enclosed in quotes
	RET

// _find_newline_delimiters_avx512(raw []byte) (mask uint64)
TEXT ·_find_newline_delimiters_avx512(SB), 7, $0
	MOVQ      raw+0(FP), SI        // SI: &raw
	MOVQ      quoteMask+24(FP), DX // get quotemask
	VMOVDQU32 (SI), Z8             // load 64 bytes

	CALL ·__init_newline_delimiters_avx512(SB)
	CALL ·__find_newline_delimiters_avx512(SB)

	MOVQ BX, mask+32(FP) // store result
	VZEROUPPER
	RET

#define NLD_CONST Z26

TEXT ·__init_newline_delimiters_avx512(SB), 7, $0
	MOVQ         $0x0a, BX     // get newline
	VPBROADCASTB BX, NLD_CONST
	RET

TEXT ·__find_newline_delimiters_avx512(SB), 7, $0
	VPCMPEQB Z8, NLD_CONST, K1
	KMOVQ    K1, BX
	ANDNQ    BX, DX, BX        // clear out newline delimiters enclosed in quotes
	RET
