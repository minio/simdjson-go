//+build !noasm !appengine

// _find_newline_delimiters(raw []byte) (mask uint64)
TEXT ·_find_newline_delimiters(SB), 7, $0
	MOVQ    raw+0(FP), SI      // SI: &raw
    VMOVDQU (SI), Y8      // load low 32-bytes
    VMOVDQU 0x20(SI), Y9  // load high 32-bytes

    CALL ·__find_newline_delimiters(SB)

	MOVQ    AX, mask+24(FP)    // store result
	VZEROUPPER
    RET

TEXT ·__find_newline_delimiters(SB), 7, $0
	MOVQ         $0x0a, AX // get newline
	MOVQ         AX, X11
	VPBROADCASTB X11, Y11

	VPCMPEQB  Y8, Y11, Y10
	VPCMPEQB  Y9, Y11, Y11
	VPMOVMSKB Y10, AX
	VPMOVMSKB Y11, CX
	SHLQ      $32, CX
	ORQ       CX, AX
	RET
