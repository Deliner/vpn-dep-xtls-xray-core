#include <assert.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

uint32_t SuperFastHash(const char *, int);

int main(int argc, char **argv) {
    (void)argv;
    if (argc > 1) {
        char *short_input = malloc(1);
        assert(short_input);
        short_input[0] = 'a';
        /* Deliberate caller violation: prove ASan detects this exact boundary. */
        volatile uint32_t value = SuperFastHash(short_input, 2);
        free(short_input);
        return value == 0;
    }
    assert(SuperFastHash(NULL, 0) == 0);
    assert(SuperFastHash("x", -1) == 0);
    unsigned cases = 2;
    for (int length = 1; length <= 4096; ++length) {
        char *reference = malloc((size_t)length);
        assert(reference);
        for (int i = 0; i < length; ++i) reference[i] = (char)(i * 37 + length);
        uint32_t expected = SuperFastHash(reference, length);
        for (size_t offset = 0; offset < 8; ++offset) {
            char *allocation = malloc((size_t)length + offset);
            assert(allocation);
            char *input = allocation + offset;
            for (int i = 0; i < length; ++i) input[i] = reference[i];
            assert(SuperFastHash(input, length) == expected);
            free(allocation);
            ++cases;
        }
        free(reference);
    }
    printf("hash_bounds_cases=%u\n", cases);
    return 0;
}
