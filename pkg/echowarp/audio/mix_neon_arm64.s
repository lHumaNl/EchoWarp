//go:build arm64 && gc && !noasm

// NEON SIMD implementations for ARM64 audio mixing.
// ARMv8 guarantees NEON support, so no runtime detection needed.
// Uses 128-bit V registers to process 4 float32 values per iteration.
//
// Go's ARM64 assembler lacks named instructions for vector float ops
// (FADD.4S, FMUL.4S, FDIV.4S), so we use WORD encoding with raw opcodes.
//
// ARM64 SIMD float instruction encoding (4S = Q=1, sz=0):
//   FADD Vd.4S, Vn.4S, Vm.4S: 0x4E20D400 | (Rm<<16) | (Rn<<5) | Rd
//   FMUL Vd.4S, Vn.4S, Vm.4S: 0x6E20DC00 | (Rm<<16) | (Rn<<5) | Rd
//   FDIV Vd.4S, Vn.4S, Vm.4S: 0x6E20FC00 | (Rm<<16) | (Rn<<5) | Rd
//   DUP  Vd.4S, Vn.S[0]:      0x4E040400 | (Rn<<5) | Rd
//
// Tanh uses Pade approximant: tanh(x) ≈ x * (27 + x²) / (27 + 9*x²)

#include "textflag.h"

// FADD Vd.4S, Vn.4S, Vm.4S = 0x4E20D400 | (Rm<<16) | (Rn<<5) | Rd
// FMUL Vd.4S, Vn.4S, Vm.4S = 0x6E20DC00 | (Rm<<16) | (Rn<<5) | Rd
// FDIV Vd.4S, Vn.4S, Vm.4S = 0x6E20FC00 | (Rm<<16) | (Rn<<5) | Rd
// DUP  Vd.4S, Vn.S[0]      = 0x4E040400 | (Rn<<5)  | Rd

// Register assignments for tanh constants:
// V5 = 27.0 broadcast, V6 = 9.0 broadcast

// func mixAccumulateNEON(dst, src []float32)
// dst[i] += src[i] for all i
TEXT ·mixAccumulateNEON(SB),NOSPLIT,$0
	MOVD dst_base+0(FP), R0
	MOVD src_base+24(FP), R1
	MOVD dst_len+8(FP), R2
	MOVD src_len+32(FP), R3

	// min(len(dst), len(src))
	CMP R2, R3
	CSEL LT, R3, R2, R2

	// Process 4 floats at a time
	LSR $2, R2, R4
	CBZ R4, acc_tail

acc_loop:
	VLD1 (R0), [V0.S4]      // V0 = dst[i:i+4]
	VLD1 (R1), [V1.S4]      // V1 = src[i:i+4]
	// FADD V2.4S, V0.4S, V1.4S  (Rd=2, Rn=0, Rm=1)
	WORD $0x4E21D402
	VST1 [V2.S4], (R0)

	ADD $16, R0
	ADD $16, R1
	SUB $1, R4
	CBNZ R4, acc_loop

acc_tail:
	AND $3, R2, R4
	CBZ R4, acc_done

acc_tail_loop:
	FMOVS (R0), F0
	FMOVS (R1), F1
	FADDS F0, F1, F2
	FMOVS F2, (R0)

	ADD $4, R0
	ADD $4, R1
	SUB $1, R4
	CBNZ R4, acc_tail_loop

acc_done:
	RET

// func mixGainNEON(dst []float32, gain float32)
// dst[i] *= gain for all i
TEXT ·mixGainNEON(SB),NOSPLIT,$0
	MOVD dst_base+0(FP), R0
	MOVD dst_len+8(FP), R1
	FMOVS gain+24(FP), F0

	// Broadcast F0 (=V0.S[0]) to V0.4S: DUP V0.4S, V0.S[0]
	// DUP Vd.4S, Vn.S[0] = 0x4E040400 | (Rn<<5) | Rd
	// Rn=0, Rd=0 => 0x4E040400
	WORD $0x4E040400

	// Process 4 floats at a time
	LSR $2, R1, R4
	CBZ R4, gain_tail

gain_loop:
	VLD1 (R0), [V1.S4]      // V1 = dst[i:i+4]
	// FMUL V1.4S, V1.4S, V0.4S  (Rd=1, Rn=1, Rm=0)
	WORD $0x6E20DC21
	VST1 [V1.S4], (R0)

	ADD $16, R0
	SUB $1, R4
	CBNZ R4, gain_loop

gain_tail:
	AND $3, R1, R4
	CBZ R4, gain_done

	// Restore scalar gain from V0.S[0]
	FMOVS (R0), F1           // dummy, we need F0 which still has gain
	// Actually F0 was overwritten by DUP. Reload gain.
	FMOVS gain+24(FP), F3

gain_tail_loop:
	FMOVS (R0), F1
	FMULS F3, F1, F2
	FMOVS F2, (R0)

	ADD $4, R0
	SUB $1, R4
	CBNZ R4, gain_tail_loop

gain_done:
	RET

// func mixTanhNEON(dst []float32)
// dst[i] = tanh(dst[i]) using Pade approximant
// tanh(x) ≈ x * (27 + x²) / (27 + 9*x²)
TEXT ·mixTanhNEON(SB),NOSPLIT,$0
	MOVD dst_base+0(FP), R0
	MOVD dst_len+8(FP), R1

	// Load 27.0 into F5, then broadcast to V5.4S
	MOVW $0x41D80000, R2     // 27.0 in IEEE754 float32
	FMOVS R2, F5
	// DUP V5.4S, V5.S[0]: 0x4E040400 | (5<<5) | 5 = 0x4E0404A5
	WORD $0x4E0404A5

	// Load 9.0 into F6, then broadcast to V6.4S
	MOVW $0x41100000, R2     // 9.0 in IEEE754 float32
	FMOVS R2, F6
	// DUP V6.4S, V6.S[0]: 0x4E040400 | (6<<5) | 6 = 0x4E0404C6
	WORD $0x4E0404C6

	// Load 1.0 into V7.4S for clamp max
	MOVW $0x3F800000, R2     // 1.0 in IEEE754 float32
	FMOVS R2, F7
	// DUP V7.4S, V7.S[0]: 0x4E040400 | (7<<5) | 7 = 0x4E0404E7
	WORD $0x4E0404E7

	// Load -1.0 into V8.4S for clamp min
	MOVW $0xBF800000, R2     // -1.0 in IEEE754 float32
	FMOVS R2, F8
	// DUP V8.4S, V8.S[0]: 0x4E040400 | (8<<5) | 8 = 0x4E040508
	WORD $0x4E040508

	// Process 4 floats at a time
	LSR $2, R1, R4
	CBZ R4, tanh_tail

tanh_loop:
	VLD1 (R0), [V0.S4]      // V0 = x

	// V1 = x * x
	// FMUL V1.4S, V0.4S, V0.4S (Rd=1, Rn=0, Rm=0)
	WORD $0x6E20DC01

	// V2 = 27 + x²
	// FADD V2.4S, V5.4S, V1.4S (Rd=2, Rn=5, Rm=1)
	WORD $0x4E21D4A2

	// V2 = x * (27 + x²)
	// FMUL V2.4S, V0.4S, V2.4S (Rd=2, Rn=0, Rm=2)
	WORD $0x6E22DC02

	// V3 = 9 * x²
	// FMUL V3.4S, V6.4S, V1.4S (Rd=3, Rn=6, Rm=1)
	WORD $0x6E21DCC3

	// V3 = 27 + 9*x²
	// FADD V3.4S, V5.4S, V3.4S (Rd=3, Rn=5, Rm=3)
	WORD $0x4E23D4A3

	// V2 = x*(27+x²) / (27+9*x²)
	// FDIV V2.4S, V2.4S, V3.4S (Rd=2, Rn=2, Rm=3)
	WORD $0x6E23FC42

	// Clamp to [-1, 1]: FMIN V2.4S, V2.4S, V7.4S (clamp <= 1.0)
	WORD $0x4EA7F442
	// FMAX V2.4S, V2.4S, V8.4S (clamp >= -1.0)
	WORD $0x4E28F442

	VST1 [V2.S4], (R0)

	ADD $16, R0
	SUB $1, R4
	CBNZ R4, tanh_loop

tanh_tail:
	AND $3, R1, R4
	CBZ R4, tanh_done

	// Reload scalar constants
	MOVW $0x41D80000, R2
	FMOVS R2, F5             // 27.0
	MOVW $0x41100000, R2
	FMOVS R2, F6             // 9.0
	MOVW $0x3F800000, R2
	FMOVS R2, F7             // 1.0
	MOVW $0xBF800000, R2
	FMOVS R2, F8             // -1.0

tanh_tail_loop:
	FMOVS (R0), F0           // x
	FMULS F0, F0, F1         // x²
	FADDS F5, F1, F2         // 27 + x²
	FMULS F0, F2, F2         // x * (27 + x²)
	FMULS F6, F1, F3         // 9 * x²
	FADDS F5, F3, F3         // 27 + 9*x²
	FDIVS F3, F2, F4         // result
	FMINS F7, F4, F4         // clamp to <= 1.0
	FMAXS F8, F4, F4         // clamp to >= -1.0
	FMOVS F4, (R0)

	ADD $4, R0
	SUB $1, R4
	CBNZ R4, tanh_tail_loop

tanh_done:
	RET
