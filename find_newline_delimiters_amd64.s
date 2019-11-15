//+build !noasm !appengine

// func find_newline_delimiters(raw []byte, indices []uint32, delimiter uint64) (rows uint64)
TEXT ·find_newline_delimiters(SB), 7, $0
	MOVQ raw+0(FP), SI      // SI: &raw
	MOVQ raw_len+8(FP), R9  // R9: len(raw)
	MOVQ indices+24(FP), DI // DI: &indices

	// TODO: Load indices_len and make sure we do not write beyond

	SHRQ $6, R9 // len(in) / 64
	CMPQ R9, $0
	JEQ  done

	MOVQ         delimiter+48(FP), AX // get newline
	MOVQ         AX, X0
	VPBROADCASTB X0, Y0
	XORQ         BX, BX

loop:
	// Scan for delimiter
	VPCMPEQB  0x00(SI)(BX*1), Y0, Y1
	VPCMPEQB  0x20(SI)(BX*1), Y0, Y2
	VPMOVMSKB Y1, AX
	VPMOVMSKB Y2, CX
	SHLQ      $32, CX
	ORQ       CX, AX
	JZ        skipCtz

loopCtz:
	TZCNTQ AX, R10
	ADDQ   $4, DI
	ADDQ   BX, R10
	BLSRQ  AX, AX
	MOVL   R10, -4(DI)
	JNZ    loopCtz

skipCtz:
	ADDQ $64, BX
	SUBQ $1, R9
	JNZ  loop

done:
	MOVQ indices+24(FP), SI // reload indices pointer
	SUBQ SI, DI
	ADDQ $4, DI             // make final pointer inclusive
	SHRQ $2, DI
	MOVQ DI, rows+56(FP)    // store result
	VZEROUPPER
	RET
