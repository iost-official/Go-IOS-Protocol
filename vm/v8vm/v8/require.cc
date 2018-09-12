#include "require.h"

#include <stdlib.h>
#include <fstream>
#include <sstream>
#include <iostream>

static char injectGasFormat[] =
    "(function(){\n"
    "const source = \"%s\";\n"
    "return injectGas(source);\n"
    "})();";
static requireFunc CRequire = nullptr;

void InitGoRequire(requireFunc require) {
    CRequire = require;
}

void NewNativeRequire(const FunctionCallbackInfo<Value> &info) {
    Isolate *isolate = info.GetIsolate();
    Local<Context> context = isolate->GetCurrentContext();
    Local<Object> global = context->Global();
    Local<Value> val = global->GetInternalField(0);
    if (!val->IsExternal()) {
        Local<Value> err = Exception::Error(
            String::NewFromUtf8(isolate, "nativeRequire val error")
        );
        isolate->ThrowException(err);
        return;
    }
    SandboxPtr sbxPtr = static_cast<SandboxPtr>(Local<External>::Cast(val)->Value());
    Sandbox *sbx = static_cast<Sandbox*>(sbxPtr);

    Local<Value> path = info[0];
    if (!path->IsString()) {
        Local<Value> err = Exception::Error(
            String::NewFromUtf8(isolate, "require empty module")
        );
        isolate->ThrowException(err);
        return;
    }

    String::Utf8Value pathStr(path);
    std::string fullRelPath = std::string(sbx->jsPath) + *pathStr + ".js";

    std::ifstream f(fullRelPath);
    std::stringstream buffer;
    buffer << f.rdbuf();

    // if it's jsFile under jsPath
    if (buffer.str().length() > 0) {
        info.GetReturnValue().Set(String::NewFromUtf8(isolate, buffer.str().c_str()));
        return;
    }

    // read go standard module again
    char *code = CRequire(sbxPtr, *pathStr);
    char *injectCode = nullptr;
    asprintf(&injectCode, injectGasFormat, code);
    free(code);

    Local<String> source = String::NewFromUtf8(isolate, injectCode, NewStringType::kNormal).ToLocalChecked();
    free(injectCode);
    Local<String> fileName = String::NewFromUtf8(isolate, *pathStr, NewStringType::kNormal).ToLocalChecked();
    Local<Script> script = Script::Compile(source, fileName);

    if (!script.IsEmpty()) {
        Local<Value> result = script->Run();
        if (!result.IsEmpty()) {
            String::Utf8Value retStr(result);
            info.GetReturnValue().Set(result);
        }
    }
}

void InitRequire(Isolate *isolate, Local<ObjectTemplate> globalTpl) {
    globalTpl->Set(
        String::NewFromUtf8(isolate, "_native_require", NewStringType::kNormal)
                      .ToLocalChecked(),
        FunctionTemplate::New(isolate, NewNativeRequire));
}
