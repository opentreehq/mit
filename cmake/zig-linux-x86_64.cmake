set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR x86_64)

get_filename_component(_toolchain_dir "${CMAKE_CURRENT_LIST_FILE}" DIRECTORY)

set(CMAKE_C_COMPILER "${_toolchain_dir}/zig-cc-linux-x86_64")
set(CMAKE_CXX_COMPILER "${_toolchain_dir}/zig-c++-linux-x86_64")
set(CMAKE_ASM_COMPILER "${_toolchain_dir}/zig-cc-linux-x86_64")

# Let cmake probe compiler features, but don't try to run cross-compiled binaries.
set(CMAKE_TRY_COMPILE_TARGET_TYPE STATIC_LIBRARY)

set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
