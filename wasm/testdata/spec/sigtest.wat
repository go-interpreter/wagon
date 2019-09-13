(module
  (type (;0;) (func (param i32) (result i32)))
  (type (;1;) (func (param i32 i32) (result i32)))
  (type (;2;) (func (result i32)))
  (type (;3;) (func (param i64)(result i64)))
  (import "env" "ontologyblockchain" (func (;0;) (type 3)))
  (import "env" "test" (global (;0;) i64))
  (func (;1;) (type 1) (param i32 i32) (result i32)
		local.get 0
		local.get 1
		i32.add
  )
  (func (;2;) (type 0) (param i32) (result i32)
		local.get 0
  )
  (func (;3;) (type 2)
		i32.const 100
		i32.const 200
		call 1
  )
  (export "invoke" (func 3))
  (global (;1;) (mut i32) (i32.const 100))
  )
