#include <stdio.h>
#include <string.h>

#include "zlib.h"
#include "zutil.h"
#include "inftrees.h"
#include "inflate.h"

/* A zlib stream containing 40961 'A' bytes. Leave one byte unread so inflate
 * saves its dictionary: a complete one-call decode may never allocate it. */
static const unsigned char compressed[] = {
    0x78, 0x9c, 0xed, 0xc1, 0x01, 0x0d, 0x00, 0x00,
    0x00, 0xc2, 0xa0, 0x6c, 0xef, 0x5f, 0xca, 0x1e,
    0x0e, 0x28, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x38, 0x30, 0x0d, 0x06, 0xa2, 0x9a
};

enum { decoded = 40960, limit = 32768, capacity = 65536 };

static int reject_header(z_stream *stream, unsigned *retained_bits) {
    unsigned char invalid[] = {0xf8, 0x00};
    unsigned char output[16];
    stream->next_in = invalid;
    stream->avail_in = sizeof(invalid);
    stream->next_out = output;
    stream->avail_out = sizeof(output);
    int result = inflate(stream, Z_NO_FLUSH);
    *retained_bits = ((struct inflate_state *)stream->state)->wbits;
    if (result != Z_DATA_ERROR || stream->total_out != 0)
        return 2;
    return inflateReset(stream) == Z_OK ? 0 : 2;
}

static int decode_partial(z_stream *stream) {
    unsigned char output[capacity];
    stream->next_in = (unsigned char *)compressed;
    stream->avail_in = sizeof(compressed);
    stream->next_out = output;
    stream->avail_out = decoded;
    int result = inflate(stream, Z_NO_FLUSH);
    if (result != Z_OK || stream->total_out != decoded || stream->avail_out != 0)
        return 2;
    for (size_t i = 0; i < decoded; i++)
        if (output[i] != 'A')
            return 2;
    return 0;
}

static int read_dictionary(z_stream *stream, unsigned *length) {
    /* Deliberately oversized: observe the contract violation without writing
     * outside our buffer, even with the vulnerable 40 KiB dictionary. */
    unsigned char dictionary[capacity];
    memset(dictionary, 0xa5, sizeof(dictionary));
    if (inflateGetDictionary(stream, dictionary, length) != Z_OK ||
        (*length != limit && *length != decoded))
        return 2;
    for (size_t i = 0; i < *length; i++)
        if (dictionary[i] != 'A')
            return 2;
    for (size_t i = *length; i < sizeof(dictionary); i++)
        if (dictionary[i] != 0xa5)
            return 2;
    return 0;
}

int main(int argc, char **argv) {
    if (argc != 2 || (strcmp(argv[1], "0") && strcmp(argv[1], "15")))
        return 2;
    int bits = strcmp(argv[1], "0") ? 15 : 0;
    unsigned retained_bits = 0, length = 0;
    z_stream stream = {0};
    if (inflateInit2(&stream, bits) != Z_OK)
        return 2;
    int result = reject_header(&stream, &retained_bits);
    if (!result)
        result = decode_partial(&stream);
    if (!result)
        result = read_dictionary(&stream, &length);
    if (!result) {
        printf("requested_bits=%d retained_bits=%u output=%lu dictionary=%u\n",
               bits, retained_bits, stream.total_out, length);
        result = length > limit ? 1 : 0;
    }
    if (inflateEnd(&stream) != Z_OK)
        result = 2;
    return result;
}
