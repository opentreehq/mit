set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR aarch64)

get_filename_component(_toolchain_dir "${CMAKE_CURRENT_LIST_FILE}" DIRECTORY)

set(CMAKE_C_COMPILER "${_toolchain_dir}/zig-cc-linux-aarch64")
set(CMAKE_CXX_COMPILER "${_toolchain_dir}/zig-c++-linux-aarch64")
set(CMAKE_ASM_COMPILER "${_toolchain_dir}/zig-cc-linux-aarch64")
set(CMAKE_AR "${_toolchain_dir}/zig-ar")
set(CMAKE_RANLIB "${_toolchain_dir}/zig-ranlib")

set(CMAKE_TRY_COMPILE_TARGET_TYPE STATIC_LIBRARY)

set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
