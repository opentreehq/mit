package llama

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-metal -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-blas
#cgo LDFLAGS: -lggml-metal -lggml-blas
#cgo LDFLAGS: -framework Foundation -framework Metal -framework MetalKit -framework Accelerate
*/
import "C"
