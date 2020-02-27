// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wast

import "fmt"

type Token struct {
	Kind   TokenKind
	Text   string
	Line   int
	Column int
	Data   interface{} // may be nil
}

func (t *Token) Copy() *Token {
	return &Token{
		Kind:   t.Kind,
		Text:   t.Text,
		Line:   t.Line,
		Column: t.Column,
		Data:   t.Data,
	}
}

func (t *Token) String() string {
	// Don't print unwanted characters (eg. newline)
	safe := ""
	for _, r := range t.Text {
		safe += safeRune(r)
	}

	switch t.Kind {
	case EOF:
		return "<EOF>"
	case STRING:
		return fmt.Sprintf("<%s \"%s\">", t.Kind, safe)
	default:
		return fmt.Sprintf("<%s '%s'>", t.Kind, safe)
	}
}

type TokenKind int

const (
	NAT TokenKind = iota
	INT
	FLOAT
	STRING
	VAR
	VALUE_TYPE
	ANYFUNC
	MUT
	LPAR
	RPAR

	NOP
	DROP
	BLOCK
	END
	IF
	THEN
	ELSE
	SELECT
	LOOP
	BR
	BR_IF
	BR_TABLE

	CALL
	CALL_INDIRECT
	RETURN
	GET_LOCAL
	SET_LOCAL
	TEE_LOCAL
	GET_GLOBAL
	SET_GLOBAL

	LOAD
	STORE
	OFFSET_EQ_NAT
	ALIGN_EQ_NAT

	CONST
	LOAD_8
	LOAD_16
	LOAD_32
	STORE_8
	STORE_16
	STORE_32

	CLZ
	CTZ
	POPCNT
	NEG
	ABS
	SQRT
	CEIL
	FLOOR
	TRUNC
	NEAREST

	ADD
	SUB
	MUL
	DIV_S
	DIV_U
	REM_S
	REM_U
	AND
	OR
	XOR
	SHL
	SHR_S
	SHR_U
	ROTL
	ROTR
	MIN
	MAX
	COPYSIGN

	EQZ
	EQ
	NE
	LT_S
	LT_U
	LE_S
	LE_U
	GT_S
	GT_U
	GE_S
	GE_U
	LT
	LE
	GT
	GE

	WRAP
	EXTEND_S
	EXTEND_U
	DEMOTE
	PROMOTE
	TRUNC_S_F32
	TRUNC_U_F32
	TRUNC_S_F64
	TRUNC_U_F64
	CONVERT_S_I32
	CONVERT_U_I32
	CONVERT_S_I64
	CONVERT_U_I64
	REINTERPRET_I32
	REINTERPRET_I64
	REINTERPRET_F32
	REINTERPRET_F64

	UNREACHABLE
	CURRENT_MEMORY
	GROW_MEMORY

	FUNC
	START
	TYPE
	PARAM
	RESULT
	LOCAL
	GLOBAL

	TABLE
	ELEM
	MEMORY
	DATA
	OFFSET
	IMPORT
	EXPORT

	MODULE
	BIN
	QUOTE

	SCRIPT
	REGISTER
	INVOKE
	GET

	ASSERT_MALFORMED
	ASSERT_INVALID
	ASSERT_SOFT_INVALID
	ASSERT_UNLINKABLE
	ASSERT_RETURN
	ASSERT_RETURN_CANONICAL_NAN
	ASSERT_RETURN_ARITHMETIC_NAN
	ASSERT_TRAP
	ASSERT_EXHAUSTION
	INPUT
	OUTPUT
	EOF
)

var tokenKindOf = map[string]TokenKind{
	"anyfunc":                      ANYFUNC,
	"mut":                          MUT,
	"nop":                          NOP,
	"drop":                         DROP,
	"block":                        BLOCK,
	"end":                          END,
	"if":                           IF,
	"then":                         THEN,
	"else":                         ELSE,
	"select":                       SELECT,
	"loop":                         LOOP,
	"br":                           BR,
	"br_if":                        BR_IF,
	"br_table":                     BR_TABLE,
	"call":                         CALL,
	"call_indirect":                CALL_INDIRECT,
	"return":                       RETURN,
	"get_local":                    GET_LOCAL,
	"set_local":                    SET_LOCAL,
	"tee_local":                    TEE_LOCAL,
	"get_global":                   GET_GLOBAL,
	"set_global":                   SET_GLOBAL,
	"unreachable":                  UNREACHABLE,
	"current_memory":               CURRENT_MEMORY,
	"grow_memory":                  GROW_MEMORY,
	"func":                         FUNC,
	"start":                        START,
	"type":                         TYPE,
	"param":                        PARAM,
	"result":                       RESULT,
	"local":                        LOCAL,
	"global":                       GLOBAL,
	"table":                        TABLE,
	"elem":                         ELEM,
	"memory":                       MEMORY,
	"data":                         DATA,
	"offset":                       OFFSET,
	"import":                       IMPORT,
	"export":                       EXPORT,
	"module":                       MODULE,
	"binary":                       BIN,
	"quote":                        QUOTE,
	"script":                       SCRIPT,
	"register":                     REGISTER,
	"invoke":                       INVOKE,
	"get":                          GET,
	"assert_malformed":             ASSERT_MALFORMED,
	"assert_invalid":               ASSERT_INVALID,
	"assert_soft_invalid":          ASSERT_SOFT_INVALID,
	"assert_unlinkabled":           ASSERT_UNLINKABLE,
	"assert_return":                ASSERT_RETURN,
	"assert_return_canonical_nan":  ASSERT_RETURN_CANONICAL_NAN,
	"assert_return_arithmetic_nan": ASSERT_RETURN_ARITHMETIC_NAN,
	"assert_trap":                  ASSERT_TRAP,
	"assert_exhaustion":            ASSERT_EXHAUSTION,
	"input":                        INPUT,
	"output":                       OUTPUT,
}

var typedKindOf = map[string]TokenKind{
	"const":           CONST,
	"load":            LOAD,
	"store":           STORE,
	"load8":           LOAD_8,
	"load16":          LOAD_16,
	"load32":          LOAD_32,
	"store8":          STORE_8,
	"store16":         STORE_16,
	"store32":         STORE_32,
	"clz":             CLZ,
	"ctz":             CTZ,
	"popcnt":          POPCNT,
	"neg":             NEG,
	"abs":             ABS,
	"sqrt":            SQRT,
	"ceil":            CEIL,
	"floor":           FLOOR,
	"trunc":           TRUNC,
	"nearest":         NEAREST,
	"add":             ADD,
	"sub":             SUB,
	"mul":             MUL,
	"div_s":           DIV_S,
	"div_u":           DIV_U,
	"rem_s":           REM_S,
	"rem_u":           REM_U,
	"and":             AND,
	"or":              OR,
	"xor":             XOR,
	"shl":             SHL,
	"shr_s":           SHR_S,
	"shr_u":           SHR_U,
	"rotl":            ROTL,
	"rort":            ROTR,
	"min":             MIN,
	"max":             MAX,
	"copysign":        COPYSIGN,
	"eqz":             EQZ,
	"eq":              EQ,
	"ne":              NE,
	"lt_s":            LT_S,
	"lt_u":            LT_U,
	"le_s":            LE_S,
	"le_u":            LE_U,
	"gt_s":            GT_S,
	"gt_u":            GT_U,
	"ge_s":            GE_S,
	"ge_u":            GE_U,
	"lt":              LT,
	"le":              LE,
	"gt":              GT,
	"ge":              GE,
	"wrap/i64":        WRAP,
	"extend_s/i32":    EXTEND_S,
	"extend_u/i32":    EXTEND_U,
	"demote/f64":      DEMOTE,
	"promote/f32":     PROMOTE,
	"trunc_s/f32":     TRUNC_S_F32,
	"trunc_u/f32":     TRUNC_U_F32,
	"trunc_s/f64":     TRUNC_S_F64,
	"trunc_u/f64":     TRUNC_U_F64,
	"convert_s/i32":   CONVERT_S_I32,
	"convert_u/i32":   CONVERT_U_I32,
	"convert_s/i64":   CONVERT_S_I64,
	"convert_u/i64":   CONVERT_U_I64,
	"reinterpret/i32": REINTERPRET_I32,
	"reinterpret/i64": REINTERPRET_I64,
	"reinterpret/f32": REINTERPRET_F32,
	"reinterpret/f64": REINTERPRET_F64,
}

var tokenStrings = [...]string{
	NAT:                          "NAT",
	INT:                          "INT",
	FLOAT:                        "FLOAT",
	STRING:                       "STRING",
	VAR:                          "VAR",
	VALUE_TYPE:                   "VALUE_TYPE",
	ANYFUNC:                      "ANYFUNC",
	MUT:                          "MUT",
	LPAR:                         "LPAR",
	RPAR:                         "RPAR",
	NOP:                          "NOP",
	DROP:                         "DROP",
	BLOCK:                        "BLOCK",
	END:                          "END",
	IF:                           "IF",
	THEN:                         "THEN",
	ELSE:                         "ELSE",
	SELECT:                       "SELECT",
	LOOP:                         "LOOP",
	BR:                           "BR",
	BR_IF:                        "BR_IF",
	BR_TABLE:                     "BR_TABLE",
	CALL:                         "CALL",
	CALL_INDIRECT:                "CALL_INDIRECT",
	RETURN:                       "RETURN",
	GET_LOCAL:                    "GET_LOCAL",
	SET_LOCAL:                    "SET_LOCAL",
	TEE_LOCAL:                    "TEE_LOCAL",
	GET_GLOBAL:                   "GET_GLOBAL",
	SET_GLOBAL:                   "SET_GLOBAL",
	LOAD:                         "LOAD",
	STORE:                        "STORE",
	OFFSET_EQ_NAT:                "OFFSET_EQ_NAT",
	ALIGN_EQ_NAT:                 "ALIGN_EQ_NAT",
	CONST:                        "CONST",
	LOAD_8:                       "LOAD_8",
	LOAD_16:                      "LOAD_16",
	LOAD_32:                      "LOAD_32",
	STORE_8:                      "STORE_8",
	STORE_16:                     "STORE_16",
	STORE_32:                     "STORE_32",
	CLZ:                          "CLZ",
	CTZ:                          "CTZ",
	POPCNT:                       "POPCNT",
	NEG:                          "NEG",
	ABS:                          "ABS",
	SQRT:                         "SQRT",
	CEIL:                         "CEIL",
	FLOOR:                        "FLOOR",
	TRUNC:                        "TRUNC",
	NEAREST:                      "NEAREST",
	ADD:                          "ADD",
	SUB:                          "SUB",
	MUL:                          "MUL",
	DIV_S:                        "DIV_S",
	DIV_U:                        "DIV_U",
	REM_S:                        "REM_S",
	REM_U:                        "REM_U",
	AND:                          "AND",
	OR:                           "OR",
	XOR:                          "XOR",
	SHL:                          "SHL",
	SHR_S:                        "SHR_S",
	SHR_U:                        "SHR_U",
	ROTL:                         "ROTL",
	ROTR:                         "ROTR",
	MIN:                          "MIN",
	MAX:                          "MAX",
	COPYSIGN:                     "COPYSIGN",
	EQZ:                          "EQZ",
	EQ:                           "EQ",
	NE:                           "NE",
	LT_S:                         "LT_S",
	LT_U:                         "LT_U",
	LE_S:                         "LE_S",
	LE_U:                         "LE_U",
	GT_S:                         "GT_S",
	GT_U:                         "GT_U",
	GE_S:                         "GE_S",
	GE_U:                         "GE_U",
	LT:                           "LT",
	LE:                           "LE",
	GT:                           "GT",
	GE:                           "GE",
	WRAP:                         "WRAP",
	EXTEND_S:                     "EXTEND_S",
	EXTEND_U:                     "EXTEND_U",
	DEMOTE:                       "DEMOTE",
	PROMOTE:                      "PROMOTE",
	TRUNC_S_F32:                  "TRUNC_S_F32",
	TRUNC_U_F32:                  "TRUNC_U_F32",
	TRUNC_S_F64:                  "TRUNC_S_F64",
	TRUNC_U_F64:                  "TRUNC_U_F64",
	CONVERT_S_I32:                "CONVERT_S_I32",
	CONVERT_U_I32:                "CONVERT_U_I32",
	CONVERT_S_I64:                "CONVERT_S_I64",
	CONVERT_U_I64:                "CONVERT_U_I64",
	REINTERPRET_I32:              "REINTERPRET_I32",
	REINTERPRET_I64:              "REINTERPRET_I64",
	REINTERPRET_F32:              "REINTERPRET_F32",
	REINTERPRET_F64:              "REINTERPRET_F64",
	UNREACHABLE:                  "UNREACHABLE",
	CURRENT_MEMORY:               "CURRENT_MEMORY",
	GROW_MEMORY:                  "GROW_MEMORY",
	FUNC:                         "FUNC",
	START:                        "START",
	TYPE:                         "TYPE",
	PARAM:                        "PARAM",
	RESULT:                       "RESULT",
	LOCAL:                        "LOCAL",
	GLOBAL:                       "GLOBAL",
	TABLE:                        "TABLE",
	ELEM:                         "ELEM",
	MEMORY:                       "MEMORY",
	DATA:                         "DATA",
	OFFSET:                       "OFFSET",
	IMPORT:                       "IMPORT",
	EXPORT:                       "EXPORT",
	MODULE:                       "MODULE",
	BIN:                          "BIN",
	QUOTE:                        "QUOTE",
	SCRIPT:                       "SCRIPT",
	REGISTER:                     "REGISTER",
	INVOKE:                       "INVOKE",
	GET:                          "GET",
	ASSERT_MALFORMED:             "ASSERT_MALFORMED",
	ASSERT_INVALID:               "ASSERT_INVALID",
	ASSERT_SOFT_INVALID:          "ASSERT_SOFT_INVALID",
	ASSERT_UNLINKABLE:            "ASSERT_UNLINKABLE",
	ASSERT_RETURN:                "ASSERT_RETURN",
	ASSERT_RETURN_CANONICAL_NAN:  "ASSERT_RETURN_CANONICAL_NAN",
	ASSERT_RETURN_ARITHMETIC_NAN: "ASSERT_RETURN_ARITHMETIC_NAN",
	ASSERT_TRAP:                  "ASSERT_TRAP",
	ASSERT_EXHAUSTION:            "ASSERT_EXHAUSTION",
	INPUT:                        "INPUT",
	OUTPUT:                       "OUTPUT",
	EOF:                          "EOF",
}

func (t TokenKind) String() string {
	return tokenStrings[t]
}
