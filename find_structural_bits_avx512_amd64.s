//+build !noasm !appengine gc

#include "common.h"

TEXT ·_find_structural_bits_avx512(SB), $0-56

	CALL ·__init_odd_backslash_sequences_avx512(SB)
	CALL ·__init_quote_mask_and_bits_avx512(SB)
	CALL ·__init_whitespace_and_structurals_avx512(SB)
	CALL ·__init_newline_delimiters_avx512(SB)

	MOVQ p1+0(FP), DI
	MOVQ p3+8(FP), DX

	KORQ K_ERRORMASK, K_ERRORMASK, K_ERRORMASK

	VMOVDQU32 (DI), Z8

	CALL ·__find_odd_backslash_sequences_avx512(SB)

	MOVQ AX, DX                            // odd_ends + 16
	MOVQ prev_iter_inside_quote+16(FP), CX

	CALL  ·__find_quote_mask_and_bits_avx512(SB)
	PUSHQ AX                                     // MOVQ AX, quote_mask + 64

	CALL ·__find_whitespace_and_structurals_avx512(SB)

	POPQ DX                                    // DX = quote_mask
	MOVQ prev_iter_ends_pseudo_pred+40(FP), R8 // R8 = &prev_iter_ends_pseudo_pred

	CALL ·__finalize_structurals_avx512(SB)

	VZEROUPPER
	MOVQ  error_mask+24(FP), R9
	KMOVQ K_ERRORMASK, (R9)
	MOVQ  AX, structurals+48(FP)
	RET

#define MASK_WHITESPACE(MAX, Y) \
	LEAQ     MASKTABLE<>(SB), DX \
	MOVQ     $MAX, BX            \
	SUBQ     CX, BX              \
	VMOVDQU  (DX)(BX*1), Y10     \ // Load mask
	VPCMPEQB Y11, Y11, Y11       \ // Set all bits
	VPXOR    Y11, Y10, Y12       \ // Invert mask
	VPAND    Y13, Y12, Y12       \ // Mask whitespace
	VPAND    Y10, Y, Y           \ // Mask message
	VPOR     Y12, Y, Y           // Combine together

TEXT ·_find_structural_bits_in_slice_avx512(SB), $0-104

	CALL ·__init_odd_backslash_sequences_avx512(SB)
	CALL ·__init_quote_mask_and_bits_avx512(SB)
	CALL ·__init_whitespace_and_structurals_avx512(SB)
	CALL ·__init_newline_delimiters_avx512(SB)

	MOVQ  error_mask+32(FP), R9
	KMOVQ (R9), K_ERRORMASK

	XORQ AX, AX
	MOVQ len+8(FP), CX
	ANDQ $0xffffffffffffffc0, CX
	CMPQ AX, CX
	JEQ  check_partial_load

loop:
	MOVQ      buf+0(FP), DI
	VMOVDQU32 (DI)(AX*1), Z8
	ADDQ      $0x40, AX

loop_after_load:
	PUSHQ CX
	PUSHQ AX

	MOVQ p3+16(FP), DX
	CALL ·__find_odd_backslash_sequences_avx512(SB)

	MOVQ AX, DX                            // odd_ends + 16
	MOVQ prev_iter_inside_quote+24(FP), CX

	CALL  ·__find_quote_mask_and_bits_avx512(SB)
	PUSHQ AX                                     // MOVQ AX, quote_mask + 64

	CALL ·__find_whitespace_and_structurals_avx512(SB)

	POPQ  DX                                    // DX = quote_mask
	PUSHQ DX                                    // Save again for newline determination
	MOVQ  prev_iter_ends_pseudo_pred+40(FP), R8 // R8 = &prev_iter_ends_pseudo_pred

	CALL ·__finalize_structurals_avx512(SB)

	POPQ DX                                    // DX = quote_mask
	CMPQ ndjson+88(FP), $0
	JZ   skip_ndjson_detection
	CALL ·__find_newline_delimiters_avx512(SB)
	ORQ  BX, AX

skip_ndjson_detection:
	MOVQ indexes+48(FP), DI
	MOVQ index+56(FP), SI; MOVQ (SI), BX       // BX = index
	MOVQ carried+72(FP), R11; MOVQ (R11), DX   // DX = carried
	MOVQ position+80(FP), R12; MOVQ (R12), R10 // R10 = position
	CALL ·__flatten_bits_incremental(SB)
	MOVQ BX, (SI)                              // *index = BX
	MOVQ DX, (R11)                             // *carried = DX
	MOVQ R10, (R12)                            // *position = R10

	POPQ AX
	POPQ CX

	CMPQ BX, indexes_len+64(FP)
	JGE  done

	CMPQ AX, CX
	JLT  loop

	// Check if AX is not aligned on a 64-byte boundary, this signals the last (partial) iteration
	MOVQ AX, BX
	ANDQ $0x3f, BX
	CMPQ BX, $0
	JNE  done

check_partial_load:
	MOVQ len+8(FP), CX
	ANDQ $0x3f, CX
	CMPQ CX, $0
	JNE  masking       // end of message is not aligned on 64-byte boundary, so mask the remaining bytes

done:
	VZEROUPPER
	MOVQ  error_mask+32(FP), R9
	KMOVQ K_ERRORMASK, (R9)
	MOVQ  AX, processed+96(FP)
	RET

masking:
	// Do a partial load and mask out bytes after the end of the message with whitespace
	VPBROADCASTQ WHITESPACE<>(SB), Y13 // Load padding whitespace constant

	MOVQ    buf+0(FP), DI
	VMOVDQU (DI)(AX*1), Y8 // Always load low 32-bytes
	CMPQ    CX, $0x20
	JGE     masking_high

	// Perform masking on low 32-bytes
	MASK_WHITESPACE(0x1f, Y8)
	VMOVDQU Y13, Y9
	JMP     masking_done

masking_high:
	// Perform masking on high 32-bytes
	VMOVDQU 0x20(DI)(AX*1), Y9 // Load high 32-bytes
	MASK_WHITESPACE(0x3f, Y9)

masking_done:
	ADDQ CX, AX

	// Merge Y9 into upper half of Z8
	VPXORD  Z10, Z10, Z10
	VALIGND $8, Z10, Z9, Z9
	VPORD   Z9, Z8, Z8

	JMP loop_after_load // Rejoin loop after regular loading

DATA MASKTABLE<>+0x000(SB)/8, $0xffffffffffffffff
DATA MASKTABLE<>+0x008(SB)/8, $0xffffffffffffffff
DATA MASKTABLE<>+0x010(SB)/8, $0xffffffffffffffff
DATA MASKTABLE<>+0x018(SB)/8, $0x00ffffffffffffff
DATA MASKTABLE<>+0x020(SB)/8, $0x0000000000000000
DATA MASKTABLE<>+0x028(SB)/8, $0x0000000000000000
DATA MASKTABLE<>+0x030(SB)/8, $0x0000000000000000
DATA MASKTABLE<>+0x038(SB)/8, $0x0000000000000000
GLOBL MASKTABLE<>(SB), 8, $64

DATA WHITESPACE<>+0x000(SB)/8, $0x2020202020202020
GLOBL WHITESPACE<>(SB), 8, $8
