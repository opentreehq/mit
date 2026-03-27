package llama

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/llama.cpp/build/src -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-cpu
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-metal -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-blas
#cgo LDFLAGS: -lllama -lggml -lggml-base -lggml-cpu -lggml-metal -lggml-blas
#cgo LDFLAGS: -framework Foundation -framework Metal -framework MetalKit -framework Accelerate
*/
import "C"
