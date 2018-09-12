#include "compile.h"
#include <cstring>

#include "console.h"
#include "require.h"

#include "bignumber.js.h"
#include "int64.js.h"
#include "utils.js.h"
#include "console.js.h"

intptr_t externalRef[] = {
        reinterpret_cast<intptr_t>(NewConsoleLog),
        reinterpret_cast<intptr_t>(NewNativeRequire),
        0};

static char injectGasFormat[] =
    "(function(){\n"
    "const source = \"%s\";\n"
    "return injectGas(source);\n"
    "})();";

static char codeFormat[] =
        "let module = {};\n"
        "module.exports = {};\n"
        "%s\n" // load BigNumber
        "let BigNumber = module.exports;\n"
        "%s\n"  // load Int64
        "%s\n"  // load util
        "%s\n"; // load console

int compile(SandboxPtr ptr, const char *code, const char **compiledCode) {
    Sandbox *sbx = static_cast<Sandbox*>(ptr);
    Isolate *isolate = sbx->isolate;

    Locker locker(isolate);
    Isolate::Scope isolate_scope(isolate);
    HandleScope handle_scope(isolate);

    Local<Context> context = sbx->context.Get(isolate);
    Context::Scope context_scope(context);

    char *injectCode = nullptr;
    asprintf(&injectCode, injectGasFormat, code);

    Local<String> source = String::NewFromUtf8(isolate, injectCode, NewStringType::kNormal).ToLocalChecked();
    free(injectCode);
    Local<String> fileName = String::NewFromUtf8(isolate, "__inject_gas.js", NewStringType::kNormal).ToLocalChecked();
    Local<Script> script = Script::Compile(source, fileName);

    if (!script.IsEmpty()) {
        Local<Value> result = script->Run();
        if (!result.IsEmpty()) {
            String::Utf8Value retStr(result);
            *compiledCode = strdup(*retStr);
            return 0;
        }
    }
    return 1;
}

CustomStartupData createStartupData() {
    char *bignumberjs = reinterpret_cast<char *>(__libjs_bignumber_js);
    char *int64js = reinterpret_cast<char *>(__libjs_int64_js);
    char *utilsjs = reinterpret_cast<char *>(__libjs_utils_js);
    char *consolejs = reinterpret_cast<char *>(__libjs_console_js);

    char *code = nullptr;
    asprintf(&code, codeFormat,
        bignumberjs,
        int64js,
        utilsjs,
        consolejs);

    StartupData blob;
    {
        SnapshotCreator creator(externalRef);
        Isolate* isolate = creator.GetIsolate();
        {
            HandleScope handle_scope(isolate);

            Local<ObjectTemplate> globalTpl = ObjectTemplate::New(isolate);
            globalTpl->SetInternalFieldCount(1);

            // add console log
            Local<FunctionTemplate> callback = FunctionTemplate::New(isolate, NewConsoleLog);
            globalTpl->Set(String::NewFromUtf8(isolate, "_cLog", NewStringType::kNormal).ToLocalChecked(), callback);

            // add require
            callback = FunctionTemplate::New(isolate, NewNativeRequire);
            globalTpl->Set(String::NewFromUtf8(isolate, "_native_require", NewStringType::kNormal).ToLocalChecked(), callback);

            Local<Context> context = Context::New(isolate, nullptr, globalTpl);
            Context::Scope context_scope(context);

            Local<String> source = String::NewFromUtf8(isolate, code, NewStringType::kNormal).ToLocalChecked();
            Local<Script> script = Script::Compile(context, source).ToLocalChecked();
            if (!script.IsEmpty()){
                script->Run();
            }

            creator.SetDefaultContext(context);
        }
        blob = creator.CreateBlob(SnapshotCreator::FunctionCodeHandling::kClear);
    }

    return CustomStartupData{blob.data, blob.raw_size};
}
