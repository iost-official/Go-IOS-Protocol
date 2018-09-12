#ifndef IOST_V8_COMPILE_H
#define IOST_V8_COMPILE_H

#include "sandbox.h"

int compile(SandboxPtr, const char *code, const char **compiledCode);
CustomStartupData createStartupData();

extern intptr_t externalRef[];

#endif // IOST_V8_COMPILE_H