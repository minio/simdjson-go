//+build !noasm !appengine

TEXT ·_find_structural_bits(SB), $0-80

    MOVQ p1+0(FP), DI
    MOVQ p3+8(FP), DX

    VMOVDQU    (DI), Y8          // load low 32-bytes
    VMOVDQU    0x20(DI), Y9      // load high 32-bytes

    CALL ·__find_odd_backslash_sequences(SB)

    MOVQ AX, DX                  // odd_ends + 16
    MOVQ prev_iter_inside_quote+16(FP), CX
    MOVQ quote_bits+24(FP), R8
    MOVQ error_mask+32(FP), R9

    CALL ·__find_quote_mask_and_bits(SB)
    PUSHQ AX                     //  MOVQ AX, quote_mask + 64

    MOVQ whitespace+40(FP), DX
    MOVQ structurals_in+48(FP), CX

    CALL ·__find_whitespace_and_structurals(SB)

    MOVQ structurals_in+48(FP), DI; MOVQ (DI), DI // DI = structurals
    MOVQ whitespace+40(FP), SI; MOVQ (SI), SI     // SI = whitespace
    POPQ DX                                       // DX = quote_mask
    MOVQ quote_bits+24(FP), CX;MOVQ (CX), CX      // CX = quote_bits
    MOVQ prev_iter_ends_pseudo_pred+56(FP), R8    // R8 = &prev_iter_ends_pseudo_pred

    CALL ·__finalize_structurals(SB)
    MOVQ AX, structurals+64(FP)

    VZEROUPPER
    RET
