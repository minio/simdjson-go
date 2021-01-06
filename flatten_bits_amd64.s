//+build !noasm !appengine gc

#define MASK    AX
#define INDEX   BX
#define ZEROS   CX
#define CARRIED DX
#define SHIFTS  R8
#define POSITION R10

TEXT ·_flatten_bits_incremental(SB), $0-40

	MOVQ base_ptr+0(FP), DI
	MOVQ pbase+8(FP), SI
	MOVQ mask+16(FP), MASK
	MOVQ carried+24(FP), R11
	MOVQ position+32(FP), R12
	MOVQ (SI), INDEX
	MOVQ (R11), CARRIED
	MOVQ (R12), POSITION
	CALL ·__flatten_bits_incremental(SB)
	MOVQ POSITION, (R12)
	MOVQ CARRIED, (R11)
	MOVQ INDEX, (SI)
	RET

TEXT ·__flatten_bits_incremental(SB), $0
	XORQ SHIFTS, SHIFTS

	// First iteration takes CARRIED into account
	TZCNTQ MASK, ZEROS
	JCS    done        // carry is set if ZEROS == 64

	// Two shifts required because maximum combined shift (63+1) exceeds 6-bits
	SHRQ $1, MASK
	SHRQ ZEROS, MASK
	INCQ ZEROS
	ADDQ ZEROS, SHIFTS
	ADDQ CARRIED, ZEROS
	MOVL ZEROS, (DI)(INDEX*4)
	ADDQ $1, INDEX
	ADDQ ZEROS, POSITION
	XORQ CARRIED, CARRIED     // Reset CARRIED to 0 (since it has been used)

loop:
	TZCNTQ MASK, ZEROS
	JCS    done        // carry is set if ZEROS == 64

	INCQ ZEROS
	SHRQ ZEROS, MASK
	ADDQ ZEROS, SHIFTS
	MOVL ZEROS, (DI)(INDEX*4)
	ADDQ $1, INDEX
	ADDQ ZEROS, POSITION
	JMP  loop

done:
	MOVQ $64, R9
	SUBQ SHIFTS, R9
	ADDQ R9, CARRIED // CARRIED += 64 - shifts (remaining empty bits to carry over to next call)
	RET
