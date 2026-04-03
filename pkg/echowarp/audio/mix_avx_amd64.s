//go:build amd64 && gc && !noasm

// AVX/SSE SIMD implementations for x86_64 audio mixing.
// AVX path: 256-bit YMM registers, 8 float32 per iteration (Sandy Bridge+).
// SSE path: 128-bit XMM registers, 4 float32 per iteration (all amd64 CPUs).
// Selected at runtime via CPU detection in mix.go.
//
// Tanh uses Pade approximant: tanh(x) ≈ x * (27 + x²) / (27 + 9*x²)

#include "textflag.h"

// AVX constants (256-bit)
DATA tanh_avx_27<>+0(SB)/4, $0x41d80000  // 27.0
DATA tanh_avx_27<>+4(SB)/4, $0x41d80000
DATA tanh_avx_27<>+8(SB)/4, $0x41d80000
DATA tanh_avx_27<>+12(SB)/4, $0x41d80000
DATA tanh_avx_27<>+16(SB)/4, $0x41d80000
DATA tanh_avx_27<>+20(SB)/4, $0x41d80000
DATA tanh_avx_27<>+24(SB)/4, $0x41d80000
DATA tanh_avx_27<>+28(SB)/4, $0x41d80000
GLOBL tanh_avx_27<>(SB), (NOPTR+RODATA), $32

DATA tanh_avx_9<>+0(SB)/4, $0x41100000   // 9.0
DATA tanh_avx_9<>+4(SB)/4, $0x41100000
DATA tanh_avx_9<>+8(SB)/4, $0x41100000
DATA tanh_avx_9<>+12(SB)/4, $0x41100000
DATA tanh_avx_9<>+16(SB)/4, $0x41100000
DATA tanh_avx_9<>+20(SB)/4, $0x41100000
DATA tanh_avx_9<>+24(SB)/4, $0x41100000
DATA tanh_avx_9<>+28(SB)/4, $0x41100000
GLOBL tanh_avx_9<>(SB), (NOPTR+RODATA), $32

// AVX clamp constants (256-bit)
DATA clamp_avx_pos1<>+0(SB)/4, $0x3F800000  // 1.0
DATA clamp_avx_pos1<>+4(SB)/4, $0x3F800000
DATA clamp_avx_pos1<>+8(SB)/4, $0x3F800000
DATA clamp_avx_pos1<>+12(SB)/4, $0x3F800000
DATA clamp_avx_pos1<>+16(SB)/4, $0x3F800000
DATA clamp_avx_pos1<>+20(SB)/4, $0x3F800000
DATA clamp_avx_pos1<>+24(SB)/4, $0x3F800000
DATA clamp_avx_pos1<>+28(SB)/4, $0x3F800000
GLOBL clamp_avx_pos1<>(SB), (NOPTR+RODATA), $32

DATA clamp_avx_neg1<>+0(SB)/4, $0xBF800000  // -1.0
DATA clamp_avx_neg1<>+4(SB)/4, $0xBF800000
DATA clamp_avx_neg1<>+8(SB)/4, $0xBF800000
DATA clamp_avx_neg1<>+12(SB)/4, $0xBF800000
DATA clamp_avx_neg1<>+16(SB)/4, $0xBF800000
DATA clamp_avx_neg1<>+20(SB)/4, $0xBF800000
DATA clamp_avx_neg1<>+24(SB)/4, $0xBF800000
DATA clamp_avx_neg1<>+28(SB)/4, $0xBF800000
GLOBL clamp_avx_neg1<>(SB), (NOPTR+RODATA), $32

// SSE constants (128-bit)
DATA tanh_sse_27<>+0(SB)/4, $0x41d80000
DATA tanh_sse_27<>+4(SB)/4, $0x41d80000
DATA tanh_sse_27<>+8(SB)/4, $0x41d80000
DATA tanh_sse_27<>+12(SB)/4, $0x41d80000
GLOBL tanh_sse_27<>(SB), (NOPTR+RODATA), $16

DATA tanh_sse_9<>+0(SB)/4, $0x41100000
DATA tanh_sse_9<>+4(SB)/4, $0x41100000
DATA tanh_sse_9<>+8(SB)/4, $0x41100000
DATA tanh_sse_9<>+12(SB)/4, $0x41100000
GLOBL tanh_sse_9<>(SB), (NOPTR+RODATA), $16

// SSE clamp constants (128-bit)
DATA clamp_sse_pos1<>+0(SB)/4, $0x3F800000  // 1.0
DATA clamp_sse_pos1<>+4(SB)/4, $0x3F800000
DATA clamp_sse_pos1<>+8(SB)/4, $0x3F800000
DATA clamp_sse_pos1<>+12(SB)/4, $0x3F800000
GLOBL clamp_sse_pos1<>(SB), (NOPTR+RODATA), $16

DATA clamp_sse_neg1<>+0(SB)/4, $0xBF800000  // -1.0
DATA clamp_sse_neg1<>+4(SB)/4, $0xBF800000
DATA clamp_sse_neg1<>+8(SB)/4, $0xBF800000
DATA clamp_sse_neg1<>+12(SB)/4, $0xBF800000
GLOBL clamp_sse_neg1<>(SB), (NOPTR+RODATA), $16

// ============================================================================
// AVX implementations (256-bit, 8 floats per iteration)
// ============================================================================

// func mixAccumulateAVX(dst, src []float32)
TEXT ·mixAccumulateAVX(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ dst_len+8(FP), CX
	MOVQ src_len+32(FP), DX

	CMPQ CX, DX
	CMOVQGT DX, CX

	MOVQ CX, AX
	SHRQ $3, AX
	TESTQ AX, AX
	JEQ avx_acc_tail

avx_acc_loop:
	VMOVUPS (SI), Y0
	VADDPS (DI), Y0, Y0
	VMOVUPS Y0, (DI)
	ADDQ $32, DI
	ADDQ $32, SI
	DECQ AX
	JNE avx_acc_loop

avx_acc_tail:
	ANDQ $7, CX
	JEQ avx_acc_done

avx_acc_tail_loop:
	MOVSS (SI), X0
	ADDSS (DI), X0
	MOVSS X0, (DI)
	ADDQ $4, DI
	ADDQ $4, SI
	DECQ CX
	JNE avx_acc_tail_loop

avx_acc_done:
	VZEROUPPER
	RET

// func mixGainAVX(dst []float32, gain float32)
TEXT ·mixGainAVX(SB), NOSPLIT, $0-28
	MOVQ dst_base+0(FP), DI
	MOVQ dst_len+8(FP), CX
	VBROADCASTSS gain+24(FP), Y0

	MOVQ CX, AX
	SHRQ $3, AX
	TESTQ AX, AX
	JEQ avx_gain_tail

avx_gain_loop:
	VMOVUPS (DI), Y1
	VMULPS Y0, Y1, Y1
	VMOVUPS Y1, (DI)
	ADDQ $32, DI
	DECQ AX
	JNE avx_gain_loop

avx_gain_tail:
	ANDQ $7, CX
	JEQ avx_gain_done

	MOVSS gain+24(FP), X0

avx_gain_tail_loop:
	MOVSS (DI), X1
	MULSS X0, X1
	MOVSS X1, (DI)
	ADDQ $4, DI
	DECQ CX
	JNE avx_gain_tail_loop

avx_gain_done:
	VZEROUPPER
	RET

// func mixTanhAVX(dst []float32)
TEXT ·mixTanhAVX(SB), NOSPLIT, $0-24
	MOVQ dst_base+0(FP), DI
	MOVQ dst_len+8(FP), CX

	VMOVUPS tanh_avx_27<>(SB), Y3
	VMOVUPS tanh_avx_9<>(SB), Y4
	VMOVUPS clamp_avx_pos1<>(SB), Y6
	VMOVUPS clamp_avx_neg1<>(SB), Y7

	MOVQ CX, AX
	SHRQ $3, AX
	TESTQ AX, AX
	JEQ avx_tanh_tail

avx_tanh_loop:
	VMOVUPS (DI), Y0           // x
	VMULPS Y0, Y0, Y1         // x²
	VADDPS Y3, Y1, Y2         // 27 + x²
	VMULPS Y0, Y2, Y2         // x * (27 + x²)
	VMULPS Y4, Y1, Y5         // 9 * x²
	VADDPS Y3, Y5, Y5         // 27 + 9*x²
	VDIVPS Y5, Y2, Y2         // result
	VMINPS Y6, Y2, Y2         // clamp to <= 1.0
	VMAXPS Y7, Y2, Y2         // clamp to >= -1.0
	VMOVUPS Y2, (DI)

	ADDQ $32, DI
	DECQ AX
	JNE avx_tanh_loop

avx_tanh_tail:
	ANDQ $7, CX
	JEQ avx_tanh_done

	MOVUPS tanh_sse_27<>(SB), X3
	MOVUPS tanh_sse_9<>(SB), X4
	MOVSS clamp_sse_pos1<>(SB), X6
	MOVSS clamp_sse_neg1<>(SB), X7

avx_tanh_tail_scalar:
	MOVSS (DI), X0             // x
	MOVSS X0, X1
	MULSS X0, X1               // x²
	MOVSS X3, X2
	ADDSS X1, X2               // 27 + x²
	MULSS X0, X2               // x * (27 + x²)
	MOVSS X4, X5
	MULSS X1, X5               // 9 * x²
	ADDSS X3, X5               // 27 + 9*x²
	DIVSS X5, X2               // result
	MINSS X6, X2               // clamp to <= 1.0
	MAXSS X7, X2               // clamp to >= -1.0
	MOVSS X2, (DI)

	ADDQ $4, DI
	DECQ CX
	JNE avx_tanh_tail_scalar

avx_tanh_done:
	VZEROUPPER
	RET

// ============================================================================
// SSE implementations (128-bit, 4 floats per iteration)
// Works on ALL amd64 CPUs (SSE2 guaranteed by x86_64 spec)
// ============================================================================

// func mixAccumulateSSE(dst, src []float32)
TEXT ·mixAccumulateSSE(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ dst_len+8(FP), CX
	MOVQ src_len+32(FP), DX

	CMPQ CX, DX
	CMOVQGT DX, CX

	MOVQ CX, AX
	SHRQ $2, AX
	TESTQ AX, AX
	JEQ sse_acc_tail

sse_acc_loop:
	MOVUPS (SI), X0
	MOVUPS (DI), X1
	ADDPS X0, X1
	MOVUPS X1, (DI)
	ADDQ $16, DI
	ADDQ $16, SI
	DECQ AX
	JNE sse_acc_loop

sse_acc_tail:
	ANDQ $3, CX
	JEQ sse_acc_done

sse_acc_tail_loop:
	MOVSS (SI), X0
	ADDSS (DI), X0
	MOVSS X0, (DI)
	ADDQ $4, DI
	ADDQ $4, SI
	DECQ CX
	JNE sse_acc_tail_loop

sse_acc_done:
	RET

// func mixGainSSE(dst []float32, gain float32)
TEXT ·mixGainSSE(SB), NOSPLIT, $0-28
	MOVQ dst_base+0(FP), DI
	MOVQ dst_len+8(FP), CX
	MOVSS gain+24(FP), X0
	SHUFPS $0, X0, X0         // broadcast gain to all 4 lanes

	MOVQ CX, AX
	SHRQ $2, AX
	TESTQ AX, AX
	JEQ sse_gain_tail

sse_gain_loop:
	MOVUPS (DI), X1
	MULPS X0, X1
	MOVUPS X1, (DI)
	ADDQ $16, DI
	DECQ AX
	JNE sse_gain_loop

sse_gain_tail:
	ANDQ $3, CX
	JEQ sse_gain_done

sse_gain_tail_loop:
	MOVSS (DI), X1
	MULSS X0, X1
	MOVSS X1, (DI)
	ADDQ $4, DI
	DECQ CX
	JNE sse_gain_tail_loop

sse_gain_done:
	RET

// func mixTanhSSE(dst []float32)
TEXT ·mixTanhSSE(SB), NOSPLIT, $0-24
	MOVQ dst_base+0(FP), DI
	MOVQ dst_len+8(FP), CX

	MOVUPS tanh_sse_27<>(SB), X3
	MOVUPS tanh_sse_9<>(SB), X4
	MOVUPS clamp_sse_pos1<>(SB), X6
	MOVUPS clamp_sse_neg1<>(SB), X7

	MOVQ CX, AX
	SHRQ $2, AX
	TESTQ AX, AX
	JEQ sse_tanh_tail

sse_tanh_loop:
	MOVUPS (DI), X0            // x
	MOVAPS X0, X1
	MULPS X0, X1               // x²
	MOVAPS X3, X2
	ADDPS X1, X2               // 27 + x²
	MULPS X0, X2               // x * (27 + x²)
	MOVAPS X4, X5
	MULPS X1, X5               // 9 * x²
	ADDPS X3, X5               // 27 + 9*x²
	DIVPS X5, X2               // result
	MINPS X6, X2               // clamp to <= 1.0
	MAXPS X7, X2               // clamp to >= -1.0
	MOVUPS X2, (DI)

	ADDQ $16, DI
	DECQ AX
	JNE sse_tanh_loop

sse_tanh_tail:
	ANDQ $3, CX
	JEQ sse_tanh_done

sse_tanh_tail_loop:
	MOVSS (DI), X0
	MOVSS X0, X1
	MULSS X0, X1
	MOVSS X3, X2
	ADDSS X1, X2
	MULSS X0, X2
	MOVSS X4, X5
	MULSS X1, X5
	ADDSS X3, X5
	DIVSS X5, X2
	MINSS X6, X2              // clamp to <= 1.0
	MAXSS X7, X2              // clamp to >= -1.0
	MOVSS X2, (DI)

	ADDQ $4, DI
	DECQ CX
	JNE sse_tanh_tail_loop

sse_tanh_done:
	RET
