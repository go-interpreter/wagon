#[no_mangle]
pub extern "C" fn x2_plus_y2_minus_13(x: i64, y: i64) -> i64 {
    let x2 = x * x;
    let y2 = y * y;
    ((x2 + y2 - 13) & 0xff) + 1
}
