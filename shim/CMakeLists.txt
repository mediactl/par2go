# Copyright 2026 The Clustarr Authors.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.
cmake_minimum_required(VERSION 3.13)
project(par2go LANGUAGES C CXX)

if(NOT PAR2_SRC)
  message(FATAL_ERROR "PAR2_SRC must name a par2cmdline-turbo checkout (shim/build.sh sets it)")
endif()

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
# The static par2-turbo is linked into a shared object, so it must be PIC.
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
set(BUILD_LIB ON CACHE BOOL "" FORCE)
set(BUILD_TOOL OFF CACHE BOOL "" FORCE)
set(ENABLE_CREATOR OFF CACHE BOOL "" FORCE)
set(ENABLE_PAR1 OFF CACHE BOOL "" FORCE)

add_subdirectory(${PAR2_SRC} par2-build EXCLUDE_FROM_ALL)

# Upstream sets its definitions and options per directory, so the shim does
# not inherit them. A mismatch changes Par2Repairer's layout (HAVE_CONFIG_H
# picks the u64 typedef, _FILE_OFFSET_BITS sizes off_t) or, for -fno-rtti,
# fails to link. Read them rather than restate them.
get_directory_property(P2_DEFS DIRECTORY ${PAR2_SRC} COMPILE_DEFINITIONS)
get_directory_property(P2_OPTS DIRECTORY ${PAR2_SRC} COMPILE_OPTIONS)

add_library(par2go SHARED par2go.cpp)
target_compile_definitions(par2go PRIVATE ${P2_DEFS})
target_compile_options(par2go PRIVATE ${P2_OPTS} -fvisibility=hidden -fvisibility-inlines-hidden)
target_include_directories(par2go PRIVATE
  ${CMAKE_CURRENT_SOURCE_DIR}
  ${CMAKE_CURRENT_BINARY_DIR}/par2-build)   # config.h
target_link_libraries(par2go PRIVATE par2-turbo)
target_link_options(par2go PRIVATE
  -static-libstdc++ -static-libgcc
  -Wl,--no-undefined
  -Wl,--version-script=${CMAKE_CURRENT_SOURCE_DIR}/par2go.map)
# Relink when the version script changes; CMake cannot see it otherwise.
set_target_properties(par2go PROPERTIES OUTPUT_NAME par2go
  LINK_DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/par2go.map)
