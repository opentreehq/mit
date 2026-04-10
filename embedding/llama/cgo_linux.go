package llama

/*
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/llama.cpp/build/src -L${SRCDIR}/../../third_party/llama.cpp/build/ggml/src -L${SRCDIR}/../../third_party/llama.cpp/build/ggml/src/ggml-cpu
#cgo LDFLAGS: -Wl,--start-group -lllama -lggml -lggml-base -lggml-cpu -Wl,--end-group
#cgo LDFLAGS: -lstdc++ -lm -lpthread
*/
import "C"
