#include "icu_utf.h"
#include <cassert>
#include <cstdint>
#include <cstdio>
#include <cstdlib>

using namespace base_icu;

static bool scalar(int32_t c) {
    return c >= 0 && c <= 0x10ffff && !(c >= 0xd800 && c <= 0xdfff);
}

static void utf8_buffer(const uint8_t *bytes, int length) {
    int i = 0;
    while (i < length) {
        int previous = i;
        int32_t c;
        CBU8_NEXT(bytes, i, length, c);
        assert(i > previous && i <= length);
        assert(c == CBU_SENTINEL || scalar(c));
    }
}

static void utf16_buffer(const uint16_t *units, int length) {
    int i = 0;
    while (i < length) {
        int previous = i;
        int32_t c;
        CBU16_NEXT(units, i, length, c);
        assert(i > previous && i <= length && c >= 0 && c <= 0x10ffff);
    }
}

static unsigned utf8_cases() {
    unsigned cases = 0;
    for (int length = 1; length <= 4; ++length) {
        uint8_t *bytes = static_cast<uint8_t *>(malloc(length));
        assert(bytes);
        unsigned count = length == 1 ? 256 : 65536;
        for (unsigned value = 0; value < count; ++value) {
            uint32_t random = value ^ 0x9e3779b9;
            for (int j = 0; j < length; ++j) {
                random = random * 1664525u + 1013904223u;
                bytes[j] = length <= 2 ? static_cast<uint8_t>(value >> (j * 8)) : random >> 24;
            }
            utf8_buffer(bytes, length);
            ++cases;
        }
        free(bytes);
    }
    return cases;
}

static unsigned utf16_cases() {
    unsigned cases = 0;
    for (int length = 1; length <= 2; ++length) {
        uint16_t *units = static_cast<uint16_t *>(malloc(length * sizeof(uint16_t)));
        assert(units);
        for (unsigned value = 0; value < 65536; ++value) {
            units[0] = static_cast<uint16_t>(value);
            if (length == 2) units[1] = static_cast<uint16_t>(value * 317u + 0xdc00u);
            utf16_buffer(units, length);
            ++cases;
        }
        free(units);
    }
    return cases;
}

static unsigned roundtrips() {
    unsigned cases = 0;
    for (int32_t c = 0; c <= 0x10ffff; ++c) {
        if (!scalar(c)) continue;
        uint8_t *bytes = static_cast<uint8_t *>(malloc(4));
        uint16_t *units = static_cast<uint16_t *>(malloc(2 * sizeof(uint16_t)));
        assert(bytes && units);
        int n8 = 0, n16 = 0;
        CBU8_APPEND_UNSAFE(bytes, n8, c);
        CBU16_APPEND_UNSAFE(units, n16, c);
        assert(n8 >= 1 && n8 <= 4 && n16 >= 1 && n16 <= 2);
        int i8 = 0, i16 = 0;
        int32_t got8, got16;
        CBU8_NEXT(bytes, i8, n8, got8);
        CBU16_NEXT(units, i16, n16, got16);
        assert(got8 == c && got16 == c && i8 == n8 && i16 == n16);
        free(bytes);
        free(units);
        ++cases;
    }
    return cases;
}

int main(int argc, char **) {
    if (argc > 1) {
        uint8_t *bytes = static_cast<uint8_t *>(malloc(1));
        assert(bytes);
        bytes[0] = 'a';
        int i = 1;
        int32_t c;
        /* Deliberate i == length caller violation, not an ICU defect. */
        CBU8_NEXT(bytes, i, 1, c);
        free(bytes);
        return c == 0;
    }
    auto n8 = utf8_cases(), n16 = utf16_cases(), nr = roundtrips();
    printf("utf8_buffers=%u utf16_buffers=%u scalar_roundtrips=%u\n", n8, n16, nr);
}
