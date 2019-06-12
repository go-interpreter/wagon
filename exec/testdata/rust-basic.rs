// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// To compile:
//   rustc --target wasm32-unknown-unknown -O --crate-type=cdylib rust-basic.rs -o rust-basic.wasm

#[no_mangle]
pub extern "C" fn x2_plus_y2_minus_13(x: u64, y: u64) -> u64 {
    let x2 = x * x;
    let y2 = y * y;
    ((x2 + y2 - 13) & 0xff) + 1
}


#[no_mangle]
pub extern "C" fn loopedArithmeticI64Benchmark(n: u64, input: u64) -> u64 {
    let mut out = input + 2;
    for x in 0..n {
        let y = (input * x / 3) * 2;
        out += (((input + 13) * input) & 0x66ff) - x;
        out += y & 0x9;
        out += (x * 4) / 3 + y << 3;
        out += (x * 5) / 2 + y << 1;
        out += (x * 6) / 6 + y << 11;
        out += (x * 4) / 3 + y << 3;
        out += (x * 5) / 2 + y << 1;
        out += (x * 6) / 6 + y << 11;
        out += (x * 4) / 3 + y << 3;
        out += (x * 5) / 2 + y << 1;
        out += (x * 6) / 6 + y << 11;
        if x > 5 {
            out -= y * 2 - 1;
        }
    }
    out
}

#[no_mangle]
pub extern "C" fn loopedArithmeticF64Benchmark(n: u64, input: f64) -> f64 {
    let mut out = input + 2f64;
    for x in 0..n {
        let y = (input * x as f64 / 3.121) * 2.003;
        out += ((input + 13.0) * input) - x as f64;
        out -= ((input + 11.0) * input) - x as f64;
        out += ((input + 14.11) * input) - x as f64;
        out -= ((input + 10.11) * input) - x as f64;
        out += ((input + 15.22) * input) - x as f64;
        out -= ((input + 9.222) * input) - x as f64;
        out += y * input;
        out *= 1.999;
        if x > 5 && y > 0.0 {
            out -= y * 2.0;
        }
    }
    out
}

#[no_mangle]
pub extern "C" fn loopedArithmeticF32Benchmark(n: u64, input: f32) -> f32 {
    let mut out = input + 2f32;
    for x in 0..n {
        let y = (input * x as f32 / 3.121) * 2.003;
        out += ((input + 13.0) * input) - x as f32;
        out -= ((input + 11.0) * input) - x as f32;
        out += ((input + 14.11) * input) - x as f32;
        out -= ((input + 10.11) * input) - x as f32;
        out += ((input + 15.22) * input) - x as f32;
        out -= ((input + 9.222) * input) - x as f32;
        out += y * input;
        out *= 1.999;
        if x > 5 && y > 0.0 {
            out -= y * 2.0;
        }
    }
    out
}
