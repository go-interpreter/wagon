module github.com/go-interpreter/wagon

go 1.12

require (
	github.com/edsrzf/mmap-go v1.0.0
	github.com/twitchyliquid64/golang-asm v0.0.0-20190126203739-365674df15fc
	golang.org/x/sys v0.0.0-20190306220234-b354f8bf4d9e // indirect
)

// users in some countries cannot access golang.org.
replace golang.org/x/sys => github.com/golang/sys v0.0.0-20190306220234-b354f8bf4d9e
