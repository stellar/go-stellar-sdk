// Minimal XDR definitions covering all type patterns for generator tests.

const MAX_NAME_LEN = 64;
const HASH_SIZE = 32;

typedef opaque Hash[HASH_SIZE];
typedef Hash AccountId;
typedef string Name<MAX_NAME_LEN>;
typedef opaque Data<256>;
typedef int Duration;

enum Color {
    COLOR_RED = 0,
    COLOR_GREEN = 1,
    COLOR_BLUE = 2
};

struct Pair {
    int x;
    unsigned hyper y;
};

struct Mixed {
    Hash hash;
    opaque payload<>;
    int count;
};

union ExtensionPoint switch (int v) {
    case 0:
        void;
};

union Value switch (Color c) {
    case COLOR_RED:
        int redValue;
    case COLOR_GREEN:
    case COLOR_BLUE:
        Pair colorPair;
};

union OptionalEntry switch (int v) {
    case 0:
        void;
    case 1:
        Mixed entry;
};

typedef Pair Pairs<10>;
typedef int Scores[5];
typedef Mixed *OptionalMixed;
