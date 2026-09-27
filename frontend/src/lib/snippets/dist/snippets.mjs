var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __commonJS = (cb, mod) => function __require() {
  return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));

// node_modules/is-regexp/index.js
var require_is_regexp = __commonJS({
  "node_modules/is-regexp/index.js"(exports, module) {
    "use strict";
    module.exports = function(re) {
      return Object.prototype.toString.call(re) === "[object RegExp]";
    };
  }
});

// node_modules/is-obj/index.js
var require_is_obj = __commonJS({
  "node_modules/is-obj/index.js"(exports, module) {
    "use strict";
    module.exports = function(x) {
      var type = typeof x;
      return x !== null && (type === "object" || type === "function");
    };
  }
});

// node_modules/get-own-enumerable-property-symbols/lib/index.js
var require_lib = __commonJS({
  "node_modules/get-own-enumerable-property-symbols/lib/index.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.default = (object) => Object.getOwnPropertySymbols(object).filter((keySymbol) => Object.prototype.propertyIsEnumerable.call(object, keySymbol));
  }
});

// node_modules/stringify-object/index.js
var require_stringify_object = __commonJS({
  "node_modules/stringify-object/index.js"(exports, module) {
    "use strict";
    var isRegexp = require_is_regexp();
    var isObj = require_is_obj();
    var getOwnEnumPropSymbols = require_lib().default;
    module.exports = (val, opts, pad) => {
      const seen = [];
      return function stringify2(val2, opts2, pad2) {
        opts2 = opts2 || {};
        opts2.indent = opts2.indent || "	";
        pad2 = pad2 || "";
        let tokens;
        if (opts2.inlineCharacterLimit === void 0) {
          tokens = {
            newLine: "\n",
            newLineOrSpace: "\n",
            pad: pad2,
            indent: pad2 + opts2.indent
          };
        } else {
          tokens = {
            newLine: "@@__STRINGIFY_OBJECT_NEW_LINE__@@",
            newLineOrSpace: "@@__STRINGIFY_OBJECT_NEW_LINE_OR_SPACE__@@",
            pad: "@@__STRINGIFY_OBJECT_PAD__@@",
            indent: "@@__STRINGIFY_OBJECT_INDENT__@@"
          };
        }
        const expandWhiteSpace = (string) => {
          if (opts2.inlineCharacterLimit === void 0) {
            return string;
          }
          const oneLined = string.replace(new RegExp(tokens.newLine, "g"), "").replace(new RegExp(tokens.newLineOrSpace, "g"), " ").replace(new RegExp(tokens.pad + "|" + tokens.indent, "g"), "");
          if (oneLined.length <= opts2.inlineCharacterLimit) {
            return oneLined;
          }
          return string.replace(new RegExp(tokens.newLine + "|" + tokens.newLineOrSpace, "g"), "\n").replace(new RegExp(tokens.pad, "g"), pad2).replace(new RegExp(tokens.indent, "g"), pad2 + opts2.indent);
        };
        if (seen.indexOf(val2) !== -1) {
          return '"[Circular]"';
        }
        if (val2 === null || val2 === void 0 || typeof val2 === "number" || typeof val2 === "boolean" || typeof val2 === "function" || typeof val2 === "symbol" || isRegexp(val2)) {
          return String(val2);
        }
        if (val2 instanceof Date) {
          return `new Date('${val2.toISOString()}')`;
        }
        if (Array.isArray(val2)) {
          if (val2.length === 0) {
            return "[]";
          }
          seen.push(val2);
          const ret = "[" + tokens.newLine + val2.map((el, i) => {
            const eol = val2.length - 1 === i ? tokens.newLine : "," + tokens.newLineOrSpace;
            let value = stringify2(el, opts2, pad2 + opts2.indent);
            if (opts2.transform) {
              value = opts2.transform(val2, i, value);
            }
            return tokens.indent + value + eol;
          }).join("") + tokens.pad + "]";
          seen.pop();
          return expandWhiteSpace(ret);
        }
        if (isObj(val2)) {
          let objKeys = Object.keys(val2).concat(getOwnEnumPropSymbols(val2));
          if (opts2.filter) {
            objKeys = objKeys.filter((el) => opts2.filter(val2, el));
          }
          if (objKeys.length === 0) {
            return "{}";
          }
          seen.push(val2);
          const ret = "{" + tokens.newLine + objKeys.map((el, i) => {
            const eol = objKeys.length - 1 === i ? tokens.newLine : "," + tokens.newLineOrSpace;
            const isSymbol = typeof el === "symbol";
            const isClassic = !isSymbol && /^[a-z$_][a-z$_0-9]*$/i.test(el);
            const key = isSymbol || isClassic ? el : stringify2(el, opts2);
            let value = stringify2(val2[el], opts2, pad2 + opts2.indent);
            if (opts2.transform) {
              value = opts2.transform(val2, el, value);
            }
            return tokens.indent + String(key) + ": " + value + eol;
          }).join("") + tokens.pad + "}";
          seen.pop();
          return expandWhiteSpace(ret);
        }
        val2 = String(val2).replace(/[\r\n]/g, (x) => x === "\n" ? "\\n" : "\\r");
        if (opts2.singleQuotes === false) {
          val2 = val2.replace(/"/g, '\\"');
          return `"${val2}"`;
        }
        val2 = val2.replace(/\\?'/g, "\\'");
        return `'${val2}'`;
      }(val, opts, pad);
    };
  }
});

// node_modules/punycode/punycode.js
var require_punycode = __commonJS({
  "node_modules/punycode/punycode.js"(exports, module) {
    /*! https://mths.be/punycode v1.4.1 by @mathias */
    (function(root) {
      var freeExports = typeof exports == "object" && exports && !exports.nodeType && exports;
      var freeModule = typeof module == "object" && module && !module.nodeType && module;
      var freeGlobal = typeof global == "object" && global;
      if (freeGlobal.global === freeGlobal || freeGlobal.window === freeGlobal || freeGlobal.self === freeGlobal) {
        root = freeGlobal;
      }
      var punycode, maxInt = 2147483647, base = 36, tMin = 1, tMax = 26, skew = 38, damp = 700, initialBias = 72, initialN = 128, delimiter = "-", regexPunycode = /^xn--/, regexNonASCII = /[^\x20-\x7E]/, regexSeparators = /[\x2E\u3002\uFF0E\uFF61]/g, errors = {
        "overflow": "Overflow: input needs wider integers to process",
        "not-basic": "Illegal input >= 0x80 (not a basic code point)",
        "invalid-input": "Invalid input"
      }, baseMinusTMin = base - tMin, floor = Math.floor, stringFromCharCode = String.fromCharCode, key;
      function error(type) {
        throw new RangeError(errors[type]);
      }
      function map(array, fn) {
        var length = array.length;
        var result = [];
        while (length--) {
          result[length] = fn(array[length]);
        }
        return result;
      }
      function mapDomain(string, fn) {
        var parts = string.split("@");
        var result = "";
        if (parts.length > 1) {
          result = parts[0] + "@";
          string = parts[1];
        }
        string = string.replace(regexSeparators, ".");
        var labels = string.split(".");
        var encoded = map(labels, fn).join(".");
        return result + encoded;
      }
      function ucs2decode(string) {
        var output = [], counter = 0, length = string.length, value, extra;
        while (counter < length) {
          value = string.charCodeAt(counter++);
          if (value >= 55296 && value <= 56319 && counter < length) {
            extra = string.charCodeAt(counter++);
            if ((extra & 64512) == 56320) {
              output.push(((value & 1023) << 10) + (extra & 1023) + 65536);
            } else {
              output.push(value);
              counter--;
            }
          } else {
            output.push(value);
          }
        }
        return output;
      }
      function ucs2encode(array) {
        return map(array, function(value) {
          var output = "";
          if (value > 65535) {
            value -= 65536;
            output += stringFromCharCode(value >>> 10 & 1023 | 55296);
            value = 56320 | value & 1023;
          }
          output += stringFromCharCode(value);
          return output;
        }).join("");
      }
      function basicToDigit(codePoint) {
        if (codePoint - 48 < 10) {
          return codePoint - 22;
        }
        if (codePoint - 65 < 26) {
          return codePoint - 65;
        }
        if (codePoint - 97 < 26) {
          return codePoint - 97;
        }
        return base;
      }
      function digitToBasic(digit, flag) {
        return digit + 22 + 75 * (digit < 26) - ((flag != 0) << 5);
      }
      function adapt(delta, numPoints, firstTime) {
        var k = 0;
        delta = firstTime ? floor(delta / damp) : delta >> 1;
        delta += floor(delta / numPoints);
        for (; delta > baseMinusTMin * tMax >> 1; k += base) {
          delta = floor(delta / baseMinusTMin);
        }
        return floor(k + (baseMinusTMin + 1) * delta / (delta + skew));
      }
      function decode(input) {
        var output = [], inputLength = input.length, out, i = 0, n = initialN, bias = initialBias, basic, j, index, oldi, w, k, digit, t, baseMinusT;
        basic = input.lastIndexOf(delimiter);
        if (basic < 0) {
          basic = 0;
        }
        for (j = 0; j < basic; ++j) {
          if (input.charCodeAt(j) >= 128) {
            error("not-basic");
          }
          output.push(input.charCodeAt(j));
        }
        for (index = basic > 0 ? basic + 1 : 0; index < inputLength; ) {
          for (oldi = i, w = 1, k = base; ; k += base) {
            if (index >= inputLength) {
              error("invalid-input");
            }
            digit = basicToDigit(input.charCodeAt(index++));
            if (digit >= base || digit > floor((maxInt - i) / w)) {
              error("overflow");
            }
            i += digit * w;
            t = k <= bias ? tMin : k >= bias + tMax ? tMax : k - bias;
            if (digit < t) {
              break;
            }
            baseMinusT = base - t;
            if (w > floor(maxInt / baseMinusT)) {
              error("overflow");
            }
            w *= baseMinusT;
          }
          out = output.length + 1;
          bias = adapt(i - oldi, out, oldi == 0);
          if (floor(i / out) > maxInt - n) {
            error("overflow");
          }
          n += floor(i / out);
          i %= out;
          output.splice(i++, 0, n);
        }
        return ucs2encode(output);
      }
      function encode(input) {
        var n, delta, handledCPCount, basicLength, bias, j, m, q, k, t, currentValue, output = [], inputLength, handledCPCountPlusOne, baseMinusT, qMinusT;
        input = ucs2decode(input);
        inputLength = input.length;
        n = initialN;
        delta = 0;
        bias = initialBias;
        for (j = 0; j < inputLength; ++j) {
          currentValue = input[j];
          if (currentValue < 128) {
            output.push(stringFromCharCode(currentValue));
          }
        }
        handledCPCount = basicLength = output.length;
        if (basicLength) {
          output.push(delimiter);
        }
        while (handledCPCount < inputLength) {
          for (m = maxInt, j = 0; j < inputLength; ++j) {
            currentValue = input[j];
            if (currentValue >= n && currentValue < m) {
              m = currentValue;
            }
          }
          handledCPCountPlusOne = handledCPCount + 1;
          if (m - n > floor((maxInt - delta) / handledCPCountPlusOne)) {
            error("overflow");
          }
          delta += (m - n) * handledCPCountPlusOne;
          n = m;
          for (j = 0; j < inputLength; ++j) {
            currentValue = input[j];
            if (currentValue < n && ++delta > maxInt) {
              error("overflow");
            }
            if (currentValue == n) {
              for (q = delta, k = base; ; k += base) {
                t = k <= bias ? tMin : k >= bias + tMax ? tMax : k - bias;
                if (q < t) {
                  break;
                }
                qMinusT = q - t;
                baseMinusT = base - t;
                output.push(
                  stringFromCharCode(digitToBasic(t + qMinusT % baseMinusT, 0))
                );
                q = floor(qMinusT / baseMinusT);
              }
              output.push(stringFromCharCode(digitToBasic(q, 0)));
              bias = adapt(delta, handledCPCountPlusOne, handledCPCount == basicLength);
              delta = 0;
              ++handledCPCount;
            }
          }
          ++delta;
          ++n;
        }
        return output.join("");
      }
      function toUnicode(input) {
        return mapDomain(input, function(string) {
          return regexPunycode.test(string) ? decode(string.slice(4).toLowerCase()) : string;
        });
      }
      function toASCII(input) {
        return mapDomain(input, function(string) {
          return regexNonASCII.test(string) ? "xn--" + encode(string) : string;
        });
      }
      punycode = {
        /**
         * A string representing the current Punycode.js version number.
         * @memberOf punycode
         * @type String
         */
        "version": "1.4.1",
        /**
         * An object of methods to convert from JavaScript's internal character
         * representation (UCS-2) to Unicode code points, and back.
         * @see <https://mathiasbynens.be/notes/javascript-encoding>
         * @memberOf punycode
         * @type Object
         */
        "ucs2": {
          "decode": ucs2decode,
          "encode": ucs2encode
        },
        "decode": decode,
        "encode": encode,
        "toASCII": toASCII,
        "toUnicode": toUnicode
      };
      if (typeof define == "function" && typeof define.amd == "object" && define.amd) {
        define("punycode", function() {
          return punycode;
        });
      } else if (freeExports && freeModule) {
        if (module.exports == freeExports) {
          freeModule.exports = punycode;
        } else {
          for (key in punycode) {
            punycode.hasOwnProperty(key) && (freeExports[key] = punycode[key]);
          }
        }
      } else {
        root.punycode = punycode;
      }
    })(exports);
  }
});

// node_modules/es-errors/type.js
var require_type = __commonJS({
  "node_modules/es-errors/type.js"(exports, module) {
    "use strict";
    module.exports = TypeError;
  }
});

// (disabled):node_modules/object-inspect/util.inspect
var require_util = __commonJS({
  "(disabled):node_modules/object-inspect/util.inspect"() {
  }
});

// node_modules/object-inspect/index.js
var require_object_inspect = __commonJS({
  "node_modules/object-inspect/index.js"(exports, module) {
    var hasMap = typeof Map === "function" && Map.prototype;
    var mapSizeDescriptor = Object.getOwnPropertyDescriptor && hasMap ? Object.getOwnPropertyDescriptor(Map.prototype, "size") : null;
    var mapSize = hasMap && mapSizeDescriptor && typeof mapSizeDescriptor.get === "function" ? mapSizeDescriptor.get : null;
    var mapForEach = hasMap && Map.prototype.forEach;
    var hasSet = typeof Set === "function" && Set.prototype;
    var setSizeDescriptor = Object.getOwnPropertyDescriptor && hasSet ? Object.getOwnPropertyDescriptor(Set.prototype, "size") : null;
    var setSize = hasSet && setSizeDescriptor && typeof setSizeDescriptor.get === "function" ? setSizeDescriptor.get : null;
    var setForEach = hasSet && Set.prototype.forEach;
    var hasWeakMap = typeof WeakMap === "function" && WeakMap.prototype;
    var weakMapHas = hasWeakMap ? WeakMap.prototype.has : null;
    var hasWeakSet = typeof WeakSet === "function" && WeakSet.prototype;
    var weakSetHas = hasWeakSet ? WeakSet.prototype.has : null;
    var hasWeakRef = typeof WeakRef === "function" && WeakRef.prototype;
    var weakRefDeref = hasWeakRef ? WeakRef.prototype.deref : null;
    var booleanValueOf = Boolean.prototype.valueOf;
    var objectToString = Object.prototype.toString;
    var functionToString = Function.prototype.toString;
    var $match = String.prototype.match;
    var $slice = String.prototype.slice;
    var $replace = String.prototype.replace;
    var $toUpperCase = String.prototype.toUpperCase;
    var $toLowerCase = String.prototype.toLowerCase;
    var $test = RegExp.prototype.test;
    var $concat = Array.prototype.concat;
    var $join = Array.prototype.join;
    var $arrSlice = Array.prototype.slice;
    var $floor = Math.floor;
    var bigIntValueOf = typeof BigInt === "function" ? BigInt.prototype.valueOf : null;
    var gOPS = Object.getOwnPropertySymbols;
    var symToString = typeof Symbol === "function" && typeof Symbol.iterator === "symbol" ? Symbol.prototype.toString : null;
    var hasShammedSymbols = typeof Symbol === "function" && typeof Symbol.iterator === "object";
    var toStringTag = typeof Symbol === "function" && Symbol.toStringTag && (typeof Symbol.toStringTag === hasShammedSymbols ? "object" : "symbol") ? Symbol.toStringTag : null;
    var isEnumerable = Object.prototype.propertyIsEnumerable;
    var gPO = (typeof Reflect === "function" ? Reflect.getPrototypeOf : Object.getPrototypeOf) || ([].__proto__ === Array.prototype ? function(O) {
      return O.__proto__;
    } : null);
    function addNumericSeparator(num, str3) {
      if (num === Infinity || num === -Infinity || num !== num || num && num > -1e3 && num < 1e3 || $test.call(/e/, str3)) {
        return str3;
      }
      var sepRegex = /[0-9](?=(?:[0-9]{3})+(?![0-9]))/g;
      if (typeof num === "number") {
        var int = num < 0 ? -$floor(-num) : $floor(num);
        if (int !== num) {
          var intStr = String(int);
          var dec = $slice.call(str3, intStr.length + 1);
          return $replace.call(intStr, sepRegex, "$&_") + "." + $replace.call($replace.call(dec, /([0-9]{3})/g, "$&_"), /_$/, "");
        }
      }
      return $replace.call(str3, sepRegex, "$&_");
    }
    var utilInspect = require_util();
    var inspectCustom = utilInspect.custom;
    var inspectSymbol = isSymbol(inspectCustom) ? inspectCustom : null;
    var quotes = {
      __proto__: null,
      "double": '"',
      single: "'"
    };
    var quoteREs = {
      __proto__: null,
      "double": /(["\\])/g,
      single: /(['\\])/g
    };
    module.exports = function inspect_(obj, options, depth, seen) {
      var opts = options || {};
      if (has(opts, "quoteStyle") && !has(quotes, opts.quoteStyle)) {
        throw new TypeError('option "quoteStyle" must be "single" or "double"');
      }
      if (has(opts, "maxStringLength") && (typeof opts.maxStringLength === "number" ? opts.maxStringLength < 0 && opts.maxStringLength !== Infinity : opts.maxStringLength !== null)) {
        throw new TypeError('option "maxStringLength", if provided, must be a positive integer, Infinity, or `null`');
      }
      var customInspect = has(opts, "customInspect") ? opts.customInspect : true;
      if (typeof customInspect !== "boolean" && customInspect !== "symbol") {
        throw new TypeError("option \"customInspect\", if provided, must be `true`, `false`, or `'symbol'`");
      }
      if (has(opts, "indent") && opts.indent !== null && opts.indent !== "	" && !(parseInt(opts.indent, 10) === opts.indent && opts.indent > 0)) {
        throw new TypeError('option "indent" must be "\\t", an integer > 0, or `null`');
      }
      if (has(opts, "numericSeparator") && typeof opts.numericSeparator !== "boolean") {
        throw new TypeError('option "numericSeparator", if provided, must be `true` or `false`');
      }
      var numericSeparator = opts.numericSeparator;
      if (typeof obj === "undefined") {
        return "undefined";
      }
      if (obj === null) {
        return "null";
      }
      if (typeof obj === "boolean") {
        return obj ? "true" : "false";
      }
      if (typeof obj === "string") {
        return inspectString(obj, opts);
      }
      if (typeof obj === "number") {
        if (obj === 0) {
          return Infinity / obj > 0 ? "0" : "-0";
        }
        var str3 = String(obj);
        return numericSeparator ? addNumericSeparator(obj, str3) : str3;
      }
      if (typeof obj === "bigint") {
        var bigIntStr = String(obj) + "n";
        return numericSeparator ? addNumericSeparator(obj, bigIntStr) : bigIntStr;
      }
      var maxDepth = typeof opts.depth === "undefined" ? 5 : opts.depth;
      if (typeof depth === "undefined") {
        depth = 0;
      }
      if (depth >= maxDepth && maxDepth > 0 && typeof obj === "object") {
        return isArray(obj) ? "[Array]" : "[Object]";
      }
      var indent = getIndent(opts, depth);
      if (typeof seen === "undefined") {
        seen = [];
      } else if (indexOf(seen, obj) >= 0) {
        return "[Circular]";
      }
      function inspect(value, from, noIndent) {
        if (from) {
          seen = $arrSlice.call(seen);
          seen.push(from);
        }
        if (noIndent) {
          var newOpts = {
            depth: opts.depth
          };
          if (has(opts, "quoteStyle")) {
            newOpts.quoteStyle = opts.quoteStyle;
          }
          return inspect_(value, newOpts, depth + 1, seen);
        }
        return inspect_(value, opts, depth + 1, seen);
      }
      if (typeof obj === "function" && !isRegExp(obj)) {
        var name = nameOf(obj);
        var keys = arrObjKeys(obj, inspect);
        return "[Function" + (name ? ": " + name : " (anonymous)") + "]" + (keys.length > 0 ? " { " + $join.call(keys, ", ") + " }" : "");
      }
      if (isSymbol(obj)) {
        var symString = hasShammedSymbols ? $replace.call(String(obj), /^(Symbol\(.*\))_[^)]*$/, "$1") : symToString.call(obj);
        return typeof obj === "object" && !hasShammedSymbols ? markBoxed(symString) : symString;
      }
      if (isElement(obj)) {
        var s = "<" + $toLowerCase.call(String(obj.nodeName));
        var attrs = obj.attributes || [];
        for (var i = 0; i < attrs.length; i++) {
          s += " " + attrs[i].name + "=" + wrapQuotes(quote2(attrs[i].value), "double", opts);
        }
        s += ">";
        if (obj.childNodes && obj.childNodes.length) {
          s += "...";
        }
        s += "</" + $toLowerCase.call(String(obj.nodeName)) + ">";
        return s;
      }
      if (isArray(obj)) {
        if (obj.length === 0) {
          return "[]";
        }
        var xs = arrObjKeys(obj, inspect);
        if (indent && !singleLineValues(xs)) {
          return "[" + indentedJoin(xs, indent) + "]";
        }
        return "[ " + $join.call(xs, ", ") + " ]";
      }
      if (isError(obj)) {
        var parts = arrObjKeys(obj, inspect);
        if (!("cause" in Error.prototype) && "cause" in obj && !isEnumerable.call(obj, "cause")) {
          return "{ [" + String(obj) + "] " + $join.call($concat.call("[cause]: " + inspect(obj.cause), parts), ", ") + " }";
        }
        if (parts.length === 0) {
          return "[" + String(obj) + "]";
        }
        return "{ [" + String(obj) + "] " + $join.call(parts, ", ") + " }";
      }
      if (typeof obj === "object" && customInspect) {
        if (inspectSymbol && typeof obj[inspectSymbol] === "function" && utilInspect) {
          return utilInspect(obj, { depth: maxDepth - depth });
        } else if (customInspect !== "symbol" && typeof obj.inspect === "function") {
          return obj.inspect();
        }
      }
      if (isMap(obj)) {
        var mapParts = [];
        if (mapForEach) {
          mapForEach.call(obj, function(value, key) {
            mapParts.push(inspect(key, obj, true) + " => " + inspect(value, obj));
          });
        }
        return collectionOf("Map", mapSize.call(obj), mapParts, indent);
      }
      if (isSet(obj)) {
        var setParts = [];
        if (setForEach) {
          setForEach.call(obj, function(value) {
            setParts.push(inspect(value, obj));
          });
        }
        return collectionOf("Set", setSize.call(obj), setParts, indent);
      }
      if (isWeakMap(obj)) {
        return weakCollectionOf("WeakMap");
      }
      if (isWeakSet(obj)) {
        return weakCollectionOf("WeakSet");
      }
      if (isWeakRef(obj)) {
        return weakCollectionOf("WeakRef");
      }
      if (isNumber(obj)) {
        return markBoxed(inspect(Number(obj)));
      }
      if (isBigInt(obj)) {
        return markBoxed(inspect(bigIntValueOf.call(obj)));
      }
      if (isBoolean(obj)) {
        return markBoxed(booleanValueOf.call(obj));
      }
      if (isString(obj)) {
        return markBoxed(inspect(String(obj)));
      }
      if (typeof window !== "undefined" && obj === window) {
        return "{ [object Window] }";
      }
      if (typeof globalThis !== "undefined" && obj === globalThis || typeof global !== "undefined" && obj === global) {
        return "{ [object globalThis] }";
      }
      if (!isDate(obj) && !isRegExp(obj)) {
        var ys = arrObjKeys(obj, inspect);
        var isPlainObject = gPO ? gPO(obj) === Object.prototype : obj instanceof Object || obj.constructor === Object;
        var protoTag = obj instanceof Object ? "" : "null prototype";
        var stringTag = !isPlainObject && toStringTag && Object(obj) === obj && toStringTag in obj ? $slice.call(toStr(obj), 8, -1) : protoTag ? "Object" : "";
        var constructorTag = isPlainObject || typeof obj.constructor !== "function" ? "" : obj.constructor.name ? obj.constructor.name + " " : "";
        var tag = constructorTag + (stringTag || protoTag ? "[" + $join.call($concat.call([], stringTag || [], protoTag || []), ": ") + "] " : "");
        if (ys.length === 0) {
          return tag + "{}";
        }
        if (indent) {
          return tag + "{" + indentedJoin(ys, indent) + "}";
        }
        return tag + "{ " + $join.call(ys, ", ") + " }";
      }
      return String(obj);
    };
    function wrapQuotes(s, defaultStyle, opts) {
      var style = opts.quoteStyle || defaultStyle;
      var quoteChar = quotes[style];
      return quoteChar + s + quoteChar;
    }
    function quote2(s) {
      return $replace.call(String(s), /"/g, "&quot;");
    }
    function canTrustToString(obj) {
      return !toStringTag || !(typeof obj === "object" && (toStringTag in obj || typeof obj[toStringTag] !== "undefined"));
    }
    function isArray(obj) {
      return toStr(obj) === "[object Array]" && canTrustToString(obj);
    }
    function isDate(obj) {
      return toStr(obj) === "[object Date]" && canTrustToString(obj);
    }
    function isRegExp(obj) {
      return toStr(obj) === "[object RegExp]" && canTrustToString(obj);
    }
    function isError(obj) {
      return toStr(obj) === "[object Error]" && canTrustToString(obj);
    }
    function isString(obj) {
      return toStr(obj) === "[object String]" && canTrustToString(obj);
    }
    function isNumber(obj) {
      return toStr(obj) === "[object Number]" && canTrustToString(obj);
    }
    function isBoolean(obj) {
      return toStr(obj) === "[object Boolean]" && canTrustToString(obj);
    }
    function isSymbol(obj) {
      if (hasShammedSymbols) {
        return obj && typeof obj === "object" && obj instanceof Symbol;
      }
      if (typeof obj === "symbol") {
        return true;
      }
      if (!obj || typeof obj !== "object" || !symToString) {
        return false;
      }
      try {
        symToString.call(obj);
        return true;
      } catch (e) {
      }
      return false;
    }
    function isBigInt(obj) {
      if (!obj || typeof obj !== "object" || !bigIntValueOf) {
        return false;
      }
      try {
        bigIntValueOf.call(obj);
        return true;
      } catch (e) {
      }
      return false;
    }
    var hasOwn = Object.prototype.hasOwnProperty || function(key) {
      return key in this;
    };
    function has(obj, key) {
      return hasOwn.call(obj, key);
    }
    function toStr(obj) {
      return objectToString.call(obj);
    }
    function nameOf(f) {
      if (f.name) {
        return f.name;
      }
      var m = $match.call(functionToString.call(f), /^function\s*([\w$]+)/);
      if (m) {
        return m[1];
      }
      return null;
    }
    function indexOf(xs, x) {
      if (xs.indexOf) {
        return xs.indexOf(x);
      }
      for (var i = 0, l = xs.length; i < l; i++) {
        if (xs[i] === x) {
          return i;
        }
      }
      return -1;
    }
    function isMap(x) {
      if (!mapSize || !x || typeof x !== "object") {
        return false;
      }
      try {
        mapSize.call(x);
        try {
          setSize.call(x);
        } catch (s) {
          return true;
        }
        return x instanceof Map;
      } catch (e) {
      }
      return false;
    }
    function isWeakMap(x) {
      if (!weakMapHas || !x || typeof x !== "object") {
        return false;
      }
      try {
        weakMapHas.call(x, weakMapHas);
        try {
          weakSetHas.call(x, weakSetHas);
        } catch (s) {
          return true;
        }
        return x instanceof WeakMap;
      } catch (e) {
      }
      return false;
    }
    function isWeakRef(x) {
      if (!weakRefDeref || !x || typeof x !== "object") {
        return false;
      }
      try {
        weakRefDeref.call(x);
        return true;
      } catch (e) {
      }
      return false;
    }
    function isSet(x) {
      if (!setSize || !x || typeof x !== "object") {
        return false;
      }
      try {
        setSize.call(x);
        try {
          mapSize.call(x);
        } catch (m) {
          return true;
        }
        return x instanceof Set;
      } catch (e) {
      }
      return false;
    }
    function isWeakSet(x) {
      if (!weakSetHas || !x || typeof x !== "object") {
        return false;
      }
      try {
        weakSetHas.call(x, weakSetHas);
        try {
          weakMapHas.call(x, weakMapHas);
        } catch (s) {
          return true;
        }
        return x instanceof WeakSet;
      } catch (e) {
      }
      return false;
    }
    function isElement(x) {
      if (!x || typeof x !== "object") {
        return false;
      }
      if (typeof HTMLElement !== "undefined" && x instanceof HTMLElement) {
        return true;
      }
      return typeof x.nodeName === "string" && typeof x.getAttribute === "function";
    }
    function inspectString(str3, opts) {
      if (str3.length > opts.maxStringLength) {
        var remaining = str3.length - opts.maxStringLength;
        var trailer = "... " + remaining + " more character" + (remaining > 1 ? "s" : "");
        return inspectString($slice.call(str3, 0, opts.maxStringLength), opts) + trailer;
      }
      var quoteRE = quoteREs[opts.quoteStyle || "single"];
      quoteRE.lastIndex = 0;
      var s = $replace.call($replace.call(str3, quoteRE, "\\$1"), /[\x00-\x1f]/g, lowbyte);
      return wrapQuotes(s, "single", opts);
    }
    function lowbyte(c2) {
      var n = c2.charCodeAt(0);
      var x = {
        8: "b",
        9: "t",
        10: "n",
        12: "f",
        13: "r"
      }[n];
      if (x) {
        return "\\" + x;
      }
      return "\\x" + (n < 16 ? "0" : "") + $toUpperCase.call(n.toString(16));
    }
    function markBoxed(str3) {
      return "Object(" + str3 + ")";
    }
    function weakCollectionOf(type) {
      return type + " { ? }";
    }
    function collectionOf(type, size, entries, indent) {
      var joinedEntries = indent ? indentedJoin(entries, indent) : $join.call(entries, ", ");
      return type + " (" + size + ") {" + joinedEntries + "}";
    }
    function singleLineValues(xs) {
      for (var i = 0; i < xs.length; i++) {
        if (indexOf(xs[i], "\n") >= 0) {
          return false;
        }
      }
      return true;
    }
    function getIndent(opts, depth) {
      var baseIndent;
      if (opts.indent === "	") {
        baseIndent = "	";
      } else if (typeof opts.indent === "number" && opts.indent > 0) {
        baseIndent = $join.call(Array(opts.indent + 1), " ");
      } else {
        return null;
      }
      return {
        base: baseIndent,
        prev: $join.call(Array(depth + 1), baseIndent)
      };
    }
    function indentedJoin(xs, indent) {
      if (xs.length === 0) {
        return "";
      }
      var lineJoiner = "\n" + indent.prev + indent.base;
      return lineJoiner + $join.call(xs, "," + lineJoiner) + "\n" + indent.prev;
    }
    function arrObjKeys(obj, inspect) {
      var isArr = isArray(obj);
      var xs = [];
      if (isArr) {
        xs.length = obj.length;
        for (var i = 0; i < obj.length; i++) {
          xs[i] = has(obj, i) ? inspect(obj[i], obj) : "";
        }
      }
      var syms = typeof gOPS === "function" ? gOPS(obj) : [];
      var symMap;
      if (hasShammedSymbols) {
        symMap = {};
        for (var k = 0; k < syms.length; k++) {
          symMap["$" + syms[k]] = syms[k];
        }
      }
      for (var key in obj) {
        if (!has(obj, key)) {
          continue;
        }
        if (isArr && String(Number(key)) === key && key < obj.length) {
          continue;
        }
        if (hasShammedSymbols && symMap["$" + key] instanceof Symbol) {
          continue;
        } else if ($test.call(/[^\w$]/, key)) {
          xs.push(inspect(key, obj) + ": " + inspect(obj[key], obj));
        } else {
          xs.push(key + ": " + inspect(obj[key], obj));
        }
      }
      if (typeof gOPS === "function") {
        for (var j = 0; j < syms.length; j++) {
          if (isEnumerable.call(obj, syms[j])) {
            xs.push("[" + inspect(syms[j]) + "]: " + inspect(obj[syms[j]], obj));
          }
        }
      }
      return xs;
    }
  }
});

// node_modules/side-channel-list/index.js
var require_side_channel_list = __commonJS({
  "node_modules/side-channel-list/index.js"(exports, module) {
    "use strict";
    var inspect = require_object_inspect();
    var $TypeError = require_type();
    var listGetNode = function(list, key, isDelete) {
      var prev = list;
      var curr;
      for (; (curr = prev.next) != null; prev = curr) {
        if (curr.key === key) {
          prev.next = curr.next;
          if (!isDelete) {
            curr.next = /** @type {NonNullable<typeof list.next>} */
            list.next;
            list.next = curr;
          }
          return curr;
        }
      }
    };
    var listGet = function(objects, key) {
      if (!objects) {
        return void 0;
      }
      var node2 = listGetNode(objects, key);
      return node2 && node2.value;
    };
    var listSet = function(objects, key, value) {
      var node2 = listGetNode(objects, key);
      if (node2) {
        node2.value = value;
      } else {
        objects.next = /** @type {import('./list.d.ts').ListNode<typeof value, typeof key>} */
        {
          // eslint-disable-line no-param-reassign, no-extra-parens
          key,
          next: objects.next,
          value
        };
      }
    };
    var listHas = function(objects, key) {
      if (!objects) {
        return false;
      }
      return !!listGetNode(objects, key);
    };
    var listDelete = function(objects, key) {
      if (objects) {
        return listGetNode(objects, key, true);
      }
    };
    module.exports = function getSideChannelList() {
      var $o;
      var channel = {
        assert: function(key) {
          if (!channel.has(key)) {
            throw new $TypeError("Side channel does not contain " + inspect(key));
          }
        },
        "delete": function(key) {
          var deletedNode = listDelete($o, key);
          if (deletedNode && $o && !$o.next) {
            $o = void 0;
          }
          return !!deletedNode;
        },
        get: function(key) {
          return listGet($o, key);
        },
        has: function(key) {
          return listHas($o, key);
        },
        set: function(key, value) {
          if (!$o) {
            $o = {
              next: void 0
            };
          }
          listSet(
            /** @type {NonNullable<typeof $o>} */
            $o,
            key,
            value
          );
        }
      };
      return channel;
    };
  }
});

// node_modules/es-object-atoms/index.js
var require_es_object_atoms = __commonJS({
  "node_modules/es-object-atoms/index.js"(exports, module) {
    "use strict";
    module.exports = Object;
  }
});

// node_modules/es-errors/index.js
var require_es_errors = __commonJS({
  "node_modules/es-errors/index.js"(exports, module) {
    "use strict";
    module.exports = Error;
  }
});

// node_modules/es-errors/eval.js
var require_eval = __commonJS({
  "node_modules/es-errors/eval.js"(exports, module) {
    "use strict";
    module.exports = EvalError;
  }
});

// node_modules/es-errors/range.js
var require_range = __commonJS({
  "node_modules/es-errors/range.js"(exports, module) {
    "use strict";
    module.exports = RangeError;
  }
});

// node_modules/es-errors/ref.js
var require_ref = __commonJS({
  "node_modules/es-errors/ref.js"(exports, module) {
    "use strict";
    module.exports = ReferenceError;
  }
});

// node_modules/es-errors/syntax.js
var require_syntax = __commonJS({
  "node_modules/es-errors/syntax.js"(exports, module) {
    "use strict";
    module.exports = SyntaxError;
  }
});

// node_modules/es-errors/uri.js
var require_uri = __commonJS({
  "node_modules/es-errors/uri.js"(exports, module) {
    "use strict";
    module.exports = URIError;
  }
});

// node_modules/math-intrinsics/abs.js
var require_abs = __commonJS({
  "node_modules/math-intrinsics/abs.js"(exports, module) {
    "use strict";
    module.exports = Math.abs;
  }
});

// node_modules/math-intrinsics/floor.js
var require_floor = __commonJS({
  "node_modules/math-intrinsics/floor.js"(exports, module) {
    "use strict";
    module.exports = Math.floor;
  }
});

// node_modules/math-intrinsics/max.js
var require_max = __commonJS({
  "node_modules/math-intrinsics/max.js"(exports, module) {
    "use strict";
    module.exports = Math.max;
  }
});

// node_modules/math-intrinsics/min.js
var require_min = __commonJS({
  "node_modules/math-intrinsics/min.js"(exports, module) {
    "use strict";
    module.exports = Math.min;
  }
});

// node_modules/math-intrinsics/pow.js
var require_pow = __commonJS({
  "node_modules/math-intrinsics/pow.js"(exports, module) {
    "use strict";
    module.exports = Math.pow;
  }
});

// node_modules/math-intrinsics/round.js
var require_round = __commonJS({
  "node_modules/math-intrinsics/round.js"(exports, module) {
    "use strict";
    module.exports = Math.round;
  }
});

// node_modules/math-intrinsics/isNaN.js
var require_isNaN = __commonJS({
  "node_modules/math-intrinsics/isNaN.js"(exports, module) {
    "use strict";
    module.exports = Number.isNaN || function isNaN2(a) {
      return a !== a;
    };
  }
});

// node_modules/math-intrinsics/sign.js
var require_sign = __commonJS({
  "node_modules/math-intrinsics/sign.js"(exports, module) {
    "use strict";
    var $isNaN = require_isNaN();
    module.exports = function sign(number) {
      if ($isNaN(number) || number === 0) {
        return number;
      }
      return number < 0 ? -1 : 1;
    };
  }
});

// node_modules/gopd/gOPD.js
var require_gOPD = __commonJS({
  "node_modules/gopd/gOPD.js"(exports, module) {
    "use strict";
    module.exports = Object.getOwnPropertyDescriptor;
  }
});

// node_modules/gopd/index.js
var require_gopd = __commonJS({
  "node_modules/gopd/index.js"(exports, module) {
    "use strict";
    var $gOPD = require_gOPD();
    if ($gOPD) {
      try {
        $gOPD([], "length");
      } catch (e) {
        $gOPD = null;
      }
    }
    module.exports = $gOPD;
  }
});

// node_modules/es-define-property/index.js
var require_es_define_property = __commonJS({
  "node_modules/es-define-property/index.js"(exports, module) {
    "use strict";
    var $defineProperty = Object.defineProperty || false;
    if ($defineProperty) {
      try {
        $defineProperty({}, "a", { value: 1 });
      } catch (e) {
        $defineProperty = false;
      }
    }
    module.exports = $defineProperty;
  }
});

// node_modules/has-symbols/shams.js
var require_shams = __commonJS({
  "node_modules/has-symbols/shams.js"(exports, module) {
    "use strict";
    module.exports = function hasSymbols() {
      if (typeof Symbol !== "function" || typeof Object.getOwnPropertySymbols !== "function") {
        return false;
      }
      if (typeof Symbol.iterator === "symbol") {
        return true;
      }
      var obj = {};
      var sym = Symbol("test");
      var symObj = Object(sym);
      if (typeof sym === "string") {
        return false;
      }
      if (Object.prototype.toString.call(sym) !== "[object Symbol]") {
        return false;
      }
      if (Object.prototype.toString.call(symObj) !== "[object Symbol]") {
        return false;
      }
      var symVal = 42;
      obj[sym] = symVal;
      for (var _ in obj) {
        return false;
      }
      if (typeof Object.keys === "function" && Object.keys(obj).length !== 0) {
        return false;
      }
      if (typeof Object.getOwnPropertyNames === "function" && Object.getOwnPropertyNames(obj).length !== 0) {
        return false;
      }
      var syms = Object.getOwnPropertySymbols(obj);
      if (syms.length !== 1 || syms[0] !== sym) {
        return false;
      }
      if (!Object.prototype.propertyIsEnumerable.call(obj, sym)) {
        return false;
      }
      if (typeof Object.getOwnPropertyDescriptor === "function") {
        var descriptor = (
          /** @type {PropertyDescriptor} */
          Object.getOwnPropertyDescriptor(obj, sym)
        );
        if (descriptor.value !== symVal || descriptor.enumerable !== true) {
          return false;
        }
      }
      return true;
    };
  }
});

// node_modules/has-symbols/index.js
var require_has_symbols = __commonJS({
  "node_modules/has-symbols/index.js"(exports, module) {
    "use strict";
    var origSymbol = typeof Symbol !== "undefined" && Symbol;
    var hasSymbolSham = require_shams();
    module.exports = function hasNativeSymbols() {
      if (typeof origSymbol !== "function") {
        return false;
      }
      if (typeof Symbol !== "function") {
        return false;
      }
      if (typeof origSymbol("foo") !== "symbol") {
        return false;
      }
      if (typeof Symbol("bar") !== "symbol") {
        return false;
      }
      return hasSymbolSham();
    };
  }
});

// node_modules/get-proto/Reflect.getPrototypeOf.js
var require_Reflect_getPrototypeOf = __commonJS({
  "node_modules/get-proto/Reflect.getPrototypeOf.js"(exports, module) {
    "use strict";
    module.exports = typeof Reflect !== "undefined" && Reflect.getPrototypeOf || null;
  }
});

// node_modules/get-proto/Object.getPrototypeOf.js
var require_Object_getPrototypeOf = __commonJS({
  "node_modules/get-proto/Object.getPrototypeOf.js"(exports, module) {
    "use strict";
    var $Object = require_es_object_atoms();
    module.exports = $Object.getPrototypeOf || null;
  }
});

// node_modules/function-bind/implementation.js
var require_implementation = __commonJS({
  "node_modules/function-bind/implementation.js"(exports, module) {
    "use strict";
    var ERROR_MESSAGE = "Function.prototype.bind called on incompatible ";
    var toStr = Object.prototype.toString;
    var max = Math.max;
    var funcType = "[object Function]";
    var concatty = function concatty2(a, b) {
      var arr = [];
      for (var i = 0; i < a.length; i += 1) {
        arr[i] = a[i];
      }
      for (var j = 0; j < b.length; j += 1) {
        arr[j + a.length] = b[j];
      }
      return arr;
    };
    var slicy = function slicy2(arrLike, offset) {
      var arr = [];
      for (var i = offset || 0, j = 0; i < arrLike.length; i += 1, j += 1) {
        arr[j] = arrLike[i];
      }
      return arr;
    };
    var joiny = function(arr, joiner) {
      var str3 = "";
      for (var i = 0; i < arr.length; i += 1) {
        str3 += arr[i];
        if (i + 1 < arr.length) {
          str3 += joiner;
        }
      }
      return str3;
    };
    module.exports = function bind(that) {
      var target = this;
      if (typeof target !== "function" || toStr.apply(target) !== funcType) {
        throw new TypeError(ERROR_MESSAGE + target);
      }
      var args = slicy(arguments, 1);
      var bound;
      var binder = function() {
        if (this instanceof bound) {
          var result = target.apply(
            this,
            concatty(args, arguments)
          );
          if (Object(result) === result) {
            return result;
          }
          return this;
        }
        return target.apply(
          that,
          concatty(args, arguments)
        );
      };
      var boundLength = max(0, target.length - args.length);
      var boundArgs = [];
      for (var i = 0; i < boundLength; i++) {
        boundArgs[i] = "$" + i;
      }
      bound = Function("binder", "return function (" + joiny(boundArgs, ",") + "){ return binder.apply(this,arguments); }")(binder);
      if (target.prototype) {
        var Empty = function Empty2() {
        };
        Empty.prototype = target.prototype;
        bound.prototype = new Empty();
        Empty.prototype = null;
      }
      return bound;
    };
  }
});

// node_modules/function-bind/index.js
var require_function_bind = __commonJS({
  "node_modules/function-bind/index.js"(exports, module) {
    "use strict";
    var implementation = require_implementation();
    module.exports = Function.prototype.bind || implementation;
  }
});

// node_modules/call-bind-apply-helpers/functionCall.js
var require_functionCall = __commonJS({
  "node_modules/call-bind-apply-helpers/functionCall.js"(exports, module) {
    "use strict";
    module.exports = Function.prototype.call;
  }
});

// node_modules/call-bind-apply-helpers/functionApply.js
var require_functionApply = __commonJS({
  "node_modules/call-bind-apply-helpers/functionApply.js"(exports, module) {
    "use strict";
    module.exports = Function.prototype.apply;
  }
});

// node_modules/call-bind-apply-helpers/reflectApply.js
var require_reflectApply = __commonJS({
  "node_modules/call-bind-apply-helpers/reflectApply.js"(exports, module) {
    "use strict";
    module.exports = typeof Reflect !== "undefined" && Reflect && Reflect.apply;
  }
});

// node_modules/call-bind-apply-helpers/actualApply.js
var require_actualApply = __commonJS({
  "node_modules/call-bind-apply-helpers/actualApply.js"(exports, module) {
    "use strict";
    var bind = require_function_bind();
    var $apply = require_functionApply();
    var $call = require_functionCall();
    var $reflectApply = require_reflectApply();
    module.exports = $reflectApply || bind.call($call, $apply);
  }
});

// node_modules/call-bind-apply-helpers/index.js
var require_call_bind_apply_helpers = __commonJS({
  "node_modules/call-bind-apply-helpers/index.js"(exports, module) {
    "use strict";
    var bind = require_function_bind();
    var $TypeError = require_type();
    var $call = require_functionCall();
    var $actualApply = require_actualApply();
    module.exports = function callBindBasic(args) {
      if (args.length < 1 || typeof args[0] !== "function") {
        throw new $TypeError("a function is required");
      }
      return $actualApply(bind, $call, args);
    };
  }
});

// node_modules/dunder-proto/get.js
var require_get = __commonJS({
  "node_modules/dunder-proto/get.js"(exports, module) {
    "use strict";
    var callBind = require_call_bind_apply_helpers();
    var gOPD = require_gopd();
    var hasProtoAccessor;
    try {
      hasProtoAccessor = /** @type {{ __proto__?: typeof Array.prototype }} */
      [].__proto__ === Array.prototype;
    } catch (e) {
      if (!e || typeof e !== "object" || !("code" in e) || e.code !== "ERR_PROTO_ACCESS") {
        throw e;
      }
    }
    var desc = !!hasProtoAccessor && gOPD && gOPD(
      Object.prototype,
      /** @type {keyof typeof Object.prototype} */
      "__proto__"
    );
    var $Object = Object;
    var $getPrototypeOf = $Object.getPrototypeOf;
    module.exports = desc && typeof desc.get === "function" ? callBind([desc.get]) : typeof $getPrototypeOf === "function" ? (
      /** @type {import('./get')} */
      function getDunder(value) {
        return $getPrototypeOf(value == null ? value : $Object(value));
      }
    ) : false;
  }
});

// node_modules/get-proto/index.js
var require_get_proto = __commonJS({
  "node_modules/get-proto/index.js"(exports, module) {
    "use strict";
    var reflectGetProto = require_Reflect_getPrototypeOf();
    var originalGetProto = require_Object_getPrototypeOf();
    var getDunderProto = require_get();
    module.exports = reflectGetProto ? function getProto(O) {
      return reflectGetProto(O);
    } : originalGetProto ? function getProto(O) {
      if (!O || typeof O !== "object" && typeof O !== "function") {
        throw new TypeError("getProto: not an object");
      }
      return originalGetProto(O);
    } : getDunderProto ? function getProto(O) {
      return getDunderProto(O);
    } : null;
  }
});

// node_modules/hasown/index.js
var require_hasown = __commonJS({
  "node_modules/hasown/index.js"(exports, module) {
    "use strict";
    var call = Function.prototype.call;
    var $hasOwn = Object.prototype.hasOwnProperty;
    var bind = require_function_bind();
    module.exports = bind.call(call, $hasOwn);
  }
});

// node_modules/get-intrinsic/index.js
var require_get_intrinsic = __commonJS({
  "node_modules/get-intrinsic/index.js"(exports, module) {
    "use strict";
    var undefined2;
    var $Object = require_es_object_atoms();
    var $Error = require_es_errors();
    var $EvalError = require_eval();
    var $RangeError = require_range();
    var $ReferenceError = require_ref();
    var $SyntaxError = require_syntax();
    var $TypeError = require_type();
    var $URIError = require_uri();
    var abs = require_abs();
    var floor = require_floor();
    var max = require_max();
    var min = require_min();
    var pow = require_pow();
    var round = require_round();
    var sign = require_sign();
    var $Function = Function;
    var getEvalledConstructor = function(expressionSyntax) {
      try {
        return $Function('"use strict"; return (' + expressionSyntax + ").constructor;")();
      } catch (e) {
      }
    };
    var $gOPD = require_gopd();
    var $defineProperty = require_es_define_property();
    var throwTypeError = function() {
      throw new $TypeError();
    };
    var ThrowTypeError = $gOPD ? function() {
      try {
        arguments.callee;
        return throwTypeError;
      } catch (calleeThrows) {
        try {
          return $gOPD(arguments, "callee").get;
        } catch (gOPDthrows) {
          return throwTypeError;
        }
      }
    }() : throwTypeError;
    var hasSymbols = require_has_symbols()();
    var getProto = require_get_proto();
    var $ObjectGPO = require_Object_getPrototypeOf();
    var $ReflectGPO = require_Reflect_getPrototypeOf();
    var $apply = require_functionApply();
    var $call = require_functionCall();
    var needsEval = {};
    var TypedArray = typeof Uint8Array === "undefined" || !getProto ? undefined2 : getProto(Uint8Array);
    var INTRINSICS = {
      __proto__: null,
      "%AggregateError%": typeof AggregateError === "undefined" ? undefined2 : AggregateError,
      "%Array%": Array,
      "%ArrayBuffer%": typeof ArrayBuffer === "undefined" ? undefined2 : ArrayBuffer,
      "%ArrayIteratorPrototype%": hasSymbols && getProto ? getProto([][Symbol.iterator]()) : undefined2,
      "%AsyncFromSyncIteratorPrototype%": undefined2,
      "%AsyncFunction%": needsEval,
      "%AsyncGenerator%": needsEval,
      "%AsyncGeneratorFunction%": needsEval,
      "%AsyncIteratorPrototype%": needsEval,
      "%Atomics%": typeof Atomics === "undefined" ? undefined2 : Atomics,
      "%BigInt%": typeof BigInt === "undefined" ? undefined2 : BigInt,
      "%BigInt64Array%": typeof BigInt64Array === "undefined" ? undefined2 : BigInt64Array,
      "%BigUint64Array%": typeof BigUint64Array === "undefined" ? undefined2 : BigUint64Array,
      "%Boolean%": Boolean,
      "%DataView%": typeof DataView === "undefined" ? undefined2 : DataView,
      "%Date%": Date,
      "%decodeURI%": decodeURI,
      "%decodeURIComponent%": decodeURIComponent,
      "%encodeURI%": encodeURI,
      "%encodeURIComponent%": encodeURIComponent,
      "%Error%": $Error,
      "%eval%": eval,
      // eslint-disable-line no-eval
      "%EvalError%": $EvalError,
      "%Float16Array%": typeof Float16Array === "undefined" ? undefined2 : Float16Array,
      "%Float32Array%": typeof Float32Array === "undefined" ? undefined2 : Float32Array,
      "%Float64Array%": typeof Float64Array === "undefined" ? undefined2 : Float64Array,
      "%FinalizationRegistry%": typeof FinalizationRegistry === "undefined" ? undefined2 : FinalizationRegistry,
      "%Function%": $Function,
      "%GeneratorFunction%": needsEval,
      "%Int8Array%": typeof Int8Array === "undefined" ? undefined2 : Int8Array,
      "%Int16Array%": typeof Int16Array === "undefined" ? undefined2 : Int16Array,
      "%Int32Array%": typeof Int32Array === "undefined" ? undefined2 : Int32Array,
      "%isFinite%": isFinite,
      "%isNaN%": isNaN,
      "%IteratorPrototype%": hasSymbols && getProto ? getProto(getProto([][Symbol.iterator]())) : undefined2,
      "%JSON%": typeof JSON === "object" ? JSON : undefined2,
      "%Map%": typeof Map === "undefined" ? undefined2 : Map,
      "%MapIteratorPrototype%": typeof Map === "undefined" || !hasSymbols || !getProto ? undefined2 : getProto((/* @__PURE__ */ new Map())[Symbol.iterator]()),
      "%Math%": Math,
      "%Number%": Number,
      "%Object%": $Object,
      "%Object.getOwnPropertyDescriptor%": $gOPD,
      "%parseFloat%": parseFloat,
      "%parseInt%": parseInt,
      "%Promise%": typeof Promise === "undefined" ? undefined2 : Promise,
      "%Proxy%": typeof Proxy === "undefined" ? undefined2 : Proxy,
      "%RangeError%": $RangeError,
      "%ReferenceError%": $ReferenceError,
      "%Reflect%": typeof Reflect === "undefined" ? undefined2 : Reflect,
      "%RegExp%": RegExp,
      "%Set%": typeof Set === "undefined" ? undefined2 : Set,
      "%SetIteratorPrototype%": typeof Set === "undefined" || !hasSymbols || !getProto ? undefined2 : getProto((/* @__PURE__ */ new Set())[Symbol.iterator]()),
      "%SharedArrayBuffer%": typeof SharedArrayBuffer === "undefined" ? undefined2 : SharedArrayBuffer,
      "%String%": String,
      "%StringIteratorPrototype%": hasSymbols && getProto ? getProto(""[Symbol.iterator]()) : undefined2,
      "%Symbol%": hasSymbols ? Symbol : undefined2,
      "%SyntaxError%": $SyntaxError,
      "%ThrowTypeError%": ThrowTypeError,
      "%TypedArray%": TypedArray,
      "%TypeError%": $TypeError,
      "%Uint8Array%": typeof Uint8Array === "undefined" ? undefined2 : Uint8Array,
      "%Uint8ClampedArray%": typeof Uint8ClampedArray === "undefined" ? undefined2 : Uint8ClampedArray,
      "%Uint16Array%": typeof Uint16Array === "undefined" ? undefined2 : Uint16Array,
      "%Uint32Array%": typeof Uint32Array === "undefined" ? undefined2 : Uint32Array,
      "%URIError%": $URIError,
      "%WeakMap%": typeof WeakMap === "undefined" ? undefined2 : WeakMap,
      "%WeakRef%": typeof WeakRef === "undefined" ? undefined2 : WeakRef,
      "%WeakSet%": typeof WeakSet === "undefined" ? undefined2 : WeakSet,
      "%Function.prototype.call%": $call,
      "%Function.prototype.apply%": $apply,
      "%Object.defineProperty%": $defineProperty,
      "%Object.getPrototypeOf%": $ObjectGPO,
      "%Math.abs%": abs,
      "%Math.floor%": floor,
      "%Math.max%": max,
      "%Math.min%": min,
      "%Math.pow%": pow,
      "%Math.round%": round,
      "%Math.sign%": sign,
      "%Reflect.getPrototypeOf%": $ReflectGPO
    };
    if (getProto) {
      try {
        null.error;
      } catch (e) {
        errorProto = getProto(getProto(e));
        INTRINSICS["%Error.prototype%"] = errorProto;
      }
    }
    var errorProto;
    var doEval = function doEval2(name) {
      var value;
      if (name === "%AsyncFunction%") {
        value = getEvalledConstructor("async function () {}");
      } else if (name === "%GeneratorFunction%") {
        value = getEvalledConstructor("function* () {}");
      } else if (name === "%AsyncGeneratorFunction%") {
        value = getEvalledConstructor("async function* () {}");
      } else if (name === "%AsyncGenerator%") {
        var fn = doEval2("%AsyncGeneratorFunction%");
        if (fn) {
          value = fn.prototype;
        }
      } else if (name === "%AsyncIteratorPrototype%") {
        var gen = doEval2("%AsyncGenerator%");
        if (gen && getProto) {
          value = getProto(gen.prototype);
        }
      }
      INTRINSICS[name] = value;
      return value;
    };
    var LEGACY_ALIASES = {
      __proto__: null,
      "%ArrayBufferPrototype%": ["ArrayBuffer", "prototype"],
      "%ArrayPrototype%": ["Array", "prototype"],
      "%ArrayProto_entries%": ["Array", "prototype", "entries"],
      "%ArrayProto_forEach%": ["Array", "prototype", "forEach"],
      "%ArrayProto_keys%": ["Array", "prototype", "keys"],
      "%ArrayProto_values%": ["Array", "prototype", "values"],
      "%AsyncFunctionPrototype%": ["AsyncFunction", "prototype"],
      "%AsyncGenerator%": ["AsyncGeneratorFunction", "prototype"],
      "%AsyncGeneratorPrototype%": ["AsyncGeneratorFunction", "prototype", "prototype"],
      "%BooleanPrototype%": ["Boolean", "prototype"],
      "%DataViewPrototype%": ["DataView", "prototype"],
      "%DatePrototype%": ["Date", "prototype"],
      "%ErrorPrototype%": ["Error", "prototype"],
      "%EvalErrorPrototype%": ["EvalError", "prototype"],
      "%Float32ArrayPrototype%": ["Float32Array", "prototype"],
      "%Float64ArrayPrototype%": ["Float64Array", "prototype"],
      "%FunctionPrototype%": ["Function", "prototype"],
      "%Generator%": ["GeneratorFunction", "prototype"],
      "%GeneratorPrototype%": ["GeneratorFunction", "prototype", "prototype"],
      "%Int8ArrayPrototype%": ["Int8Array", "prototype"],
      "%Int16ArrayPrototype%": ["Int16Array", "prototype"],
      "%Int32ArrayPrototype%": ["Int32Array", "prototype"],
      "%JSONParse%": ["JSON", "parse"],
      "%JSONStringify%": ["JSON", "stringify"],
      "%MapPrototype%": ["Map", "prototype"],
      "%NumberPrototype%": ["Number", "prototype"],
      "%ObjectPrototype%": ["Object", "prototype"],
      "%ObjProto_toString%": ["Object", "prototype", "toString"],
      "%ObjProto_valueOf%": ["Object", "prototype", "valueOf"],
      "%PromisePrototype%": ["Promise", "prototype"],
      "%PromiseProto_then%": ["Promise", "prototype", "then"],
      "%Promise_all%": ["Promise", "all"],
      "%Promise_reject%": ["Promise", "reject"],
      "%Promise_resolve%": ["Promise", "resolve"],
      "%RangeErrorPrototype%": ["RangeError", "prototype"],
      "%ReferenceErrorPrototype%": ["ReferenceError", "prototype"],
      "%RegExpPrototype%": ["RegExp", "prototype"],
      "%SetPrototype%": ["Set", "prototype"],
      "%SharedArrayBufferPrototype%": ["SharedArrayBuffer", "prototype"],
      "%StringPrototype%": ["String", "prototype"],
      "%SymbolPrototype%": ["Symbol", "prototype"],
      "%SyntaxErrorPrototype%": ["SyntaxError", "prototype"],
      "%TypedArrayPrototype%": ["TypedArray", "prototype"],
      "%TypeErrorPrototype%": ["TypeError", "prototype"],
      "%Uint8ArrayPrototype%": ["Uint8Array", "prototype"],
      "%Uint8ClampedArrayPrototype%": ["Uint8ClampedArray", "prototype"],
      "%Uint16ArrayPrototype%": ["Uint16Array", "prototype"],
      "%Uint32ArrayPrototype%": ["Uint32Array", "prototype"],
      "%URIErrorPrototype%": ["URIError", "prototype"],
      "%WeakMapPrototype%": ["WeakMap", "prototype"],
      "%WeakSetPrototype%": ["WeakSet", "prototype"]
    };
    var bind = require_function_bind();
    var hasOwn = require_hasown();
    var $concat = bind.call($call, Array.prototype.concat);
    var $spliceApply = bind.call($apply, Array.prototype.splice);
    var $replace = bind.call($call, String.prototype.replace);
    var $strSlice = bind.call($call, String.prototype.slice);
    var $exec = bind.call($call, RegExp.prototype.exec);
    var rePropName = /[^%.[\]]+|\[(?:(-?\d+(?:\.\d+)?)|(["'])((?:(?!\2)[^\\]|\\.)*?)\2)\]|(?=(?:\.|\[\])(?:\.|\[\]|%$))/g;
    var reEscapeChar = /\\(\\)?/g;
    var stringToPath = function stringToPath2(string) {
      var first = $strSlice(string, 0, 1);
      var last = $strSlice(string, -1);
      if (first === "%" && last !== "%") {
        throw new $SyntaxError("invalid intrinsic syntax, expected closing `%`");
      } else if (last === "%" && first !== "%") {
        throw new $SyntaxError("invalid intrinsic syntax, expected opening `%`");
      }
      var result = [];
      $replace(string, rePropName, function(match, number, quote2, subString) {
        result[result.length] = quote2 ? $replace(subString, reEscapeChar, "$1") : number || match;
      });
      return result;
    };
    var getBaseIntrinsic = function getBaseIntrinsic2(name, allowMissing) {
      var intrinsicName = name;
      var alias;
      if (hasOwn(LEGACY_ALIASES, intrinsicName)) {
        alias = LEGACY_ALIASES[intrinsicName];
        intrinsicName = "%" + alias[0] + "%";
      }
      if (hasOwn(INTRINSICS, intrinsicName)) {
        var value = INTRINSICS[intrinsicName];
        if (value === needsEval) {
          value = doEval(intrinsicName);
        }
        if (typeof value === "undefined" && !allowMissing) {
          throw new $TypeError("intrinsic " + name + " exists, but is not available. Please file an issue!");
        }
        return {
          alias,
          name: intrinsicName,
          value
        };
      }
      throw new $SyntaxError("intrinsic " + name + " does not exist!");
    };
    module.exports = function GetIntrinsic(name, allowMissing) {
      if (typeof name !== "string" || name.length === 0) {
        throw new $TypeError("intrinsic name must be a non-empty string");
      }
      if (arguments.length > 1 && typeof allowMissing !== "boolean") {
        throw new $TypeError('"allowMissing" argument must be a boolean');
      }
      if ($exec(/^%?[^%]*%?$/, name) === null) {
        throw new $SyntaxError("`%` may not be present anywhere but at the beginning and end of the intrinsic name");
      }
      var parts = stringToPath(name);
      var intrinsicBaseName = parts.length > 0 ? parts[0] : "";
      var intrinsic = getBaseIntrinsic("%" + intrinsicBaseName + "%", allowMissing);
      var intrinsicRealName = intrinsic.name;
      var value = intrinsic.value;
      var skipFurtherCaching = false;
      var alias = intrinsic.alias;
      if (alias) {
        intrinsicBaseName = alias[0];
        $spliceApply(parts, $concat([0, 1], alias));
      }
      for (var i = 1, isOwn = true; i < parts.length; i += 1) {
        var part = parts[i];
        var first = $strSlice(part, 0, 1);
        var last = $strSlice(part, -1);
        if ((first === '"' || first === "'" || first === "`" || (last === '"' || last === "'" || last === "`")) && first !== last) {
          throw new $SyntaxError("property names with quotes must have matching quotes");
        }
        if (part === "constructor" || !isOwn) {
          skipFurtherCaching = true;
        }
        intrinsicBaseName += "." + part;
        intrinsicRealName = "%" + intrinsicBaseName + "%";
        if (hasOwn(INTRINSICS, intrinsicRealName)) {
          value = INTRINSICS[intrinsicRealName];
        } else if (value != null) {
          if (!(part in value)) {
            if (!allowMissing) {
              throw new $TypeError("base intrinsic for " + name + " exists, but the property is not available.");
            }
            return void 0;
          }
          if ($gOPD && i + 1 >= parts.length) {
            var desc = $gOPD(value, part);
            isOwn = !!desc;
            if (isOwn && "get" in desc && !("originalValue" in desc.get)) {
              value = desc.get;
            } else {
              value = value[part];
            }
          } else {
            isOwn = hasOwn(value, part);
            value = value[part];
          }
          if (isOwn && !skipFurtherCaching) {
            INTRINSICS[intrinsicRealName] = value;
          }
        }
      }
      return value;
    };
  }
});

// node_modules/call-bound/index.js
var require_call_bound = __commonJS({
  "node_modules/call-bound/index.js"(exports, module) {
    "use strict";
    var GetIntrinsic = require_get_intrinsic();
    var callBindBasic = require_call_bind_apply_helpers();
    var $indexOf = callBindBasic([GetIntrinsic("%String.prototype.indexOf%")]);
    module.exports = function callBoundIntrinsic(name, allowMissing) {
      var intrinsic = (
        /** @type {(this: unknown, ...args: unknown[]) => unknown} */
        GetIntrinsic(name, !!allowMissing)
      );
      if (typeof intrinsic === "function" && $indexOf(name, ".prototype.") > -1) {
        return callBindBasic(
          /** @type {const} */
          [intrinsic]
        );
      }
      return intrinsic;
    };
  }
});

// node_modules/side-channel-map/index.js
var require_side_channel_map = __commonJS({
  "node_modules/side-channel-map/index.js"(exports, module) {
    "use strict";
    var GetIntrinsic = require_get_intrinsic();
    var callBound = require_call_bound();
    var inspect = require_object_inspect();
    var $TypeError = require_type();
    var $Map = GetIntrinsic("%Map%", true);
    var $mapGet = callBound("Map.prototype.get", true);
    var $mapSet = callBound("Map.prototype.set", true);
    var $mapHas = callBound("Map.prototype.has", true);
    var $mapDelete = callBound("Map.prototype.delete", true);
    var $mapSize = callBound("Map.prototype.size", true);
    module.exports = !!$Map && /** @type {Exclude<import('.'), false>} */
    function getSideChannelMap() {
      var $m;
      var channel = {
        assert: function(key) {
          if (!channel.has(key)) {
            throw new $TypeError("Side channel does not contain " + inspect(key));
          }
        },
        "delete": function(key) {
          if ($m) {
            var result = $mapDelete($m, key);
            if ($mapSize($m) === 0) {
              $m = void 0;
            }
            return result;
          }
          return false;
        },
        get: function(key) {
          if ($m) {
            return $mapGet($m, key);
          }
        },
        has: function(key) {
          if ($m) {
            return $mapHas($m, key);
          }
          return false;
        },
        set: function(key, value) {
          if (!$m) {
            $m = new $Map();
          }
          $mapSet($m, key, value);
        }
      };
      return channel;
    };
  }
});

// node_modules/side-channel-weakmap/index.js
var require_side_channel_weakmap = __commonJS({
  "node_modules/side-channel-weakmap/index.js"(exports, module) {
    "use strict";
    var GetIntrinsic = require_get_intrinsic();
    var callBound = require_call_bound();
    var inspect = require_object_inspect();
    var getSideChannelMap = require_side_channel_map();
    var $TypeError = require_type();
    var $WeakMap = GetIntrinsic("%WeakMap%", true);
    var $weakMapGet = callBound("WeakMap.prototype.get", true);
    var $weakMapSet = callBound("WeakMap.prototype.set", true);
    var $weakMapHas = callBound("WeakMap.prototype.has", true);
    var $weakMapDelete = callBound("WeakMap.prototype.delete", true);
    module.exports = $WeakMap ? (
      /** @type {Exclude<import('.'), false>} */
      function getSideChannelWeakMap() {
        var $wm;
        var $m;
        var channel = {
          assert: function(key) {
            if (!channel.has(key)) {
              throw new $TypeError("Side channel does not contain " + inspect(key));
            }
          },
          "delete": function(key) {
            if ($WeakMap && key && (typeof key === "object" || typeof key === "function")) {
              if ($wm) {
                return $weakMapDelete($wm, key);
              }
            } else if (getSideChannelMap) {
              if ($m) {
                return $m["delete"](key);
              }
            }
            return false;
          },
          get: function(key) {
            if ($WeakMap && key && (typeof key === "object" || typeof key === "function")) {
              if ($wm) {
                return $weakMapGet($wm, key);
              }
            }
            return $m && $m.get(key);
          },
          has: function(key) {
            if ($WeakMap && key && (typeof key === "object" || typeof key === "function")) {
              if ($wm) {
                return $weakMapHas($wm, key);
              }
            }
            return !!$m && $m.has(key);
          },
          set: function(key, value) {
            if ($WeakMap && key && (typeof key === "object" || typeof key === "function")) {
              if (!$wm) {
                $wm = new $WeakMap();
              }
              $weakMapSet($wm, key, value);
            } else if (getSideChannelMap) {
              if (!$m) {
                $m = getSideChannelMap();
              }
              $m.set(key, value);
            }
          }
        };
        return channel;
      }
    ) : getSideChannelMap;
  }
});

// node_modules/side-channel/index.js
var require_side_channel = __commonJS({
  "node_modules/side-channel/index.js"(exports, module) {
    "use strict";
    var $TypeError = require_type();
    var inspect = require_object_inspect();
    var getSideChannelList = require_side_channel_list();
    var getSideChannelMap = require_side_channel_map();
    var getSideChannelWeakMap = require_side_channel_weakmap();
    var makeChannel = getSideChannelWeakMap || getSideChannelMap || getSideChannelList;
    module.exports = function getSideChannel() {
      var $channelData;
      var channel = {
        assert: function(key) {
          if (!channel.has(key)) {
            var keyDesc = key && Object(key) === key ? "the given object key" : inspect(key);
            throw new $TypeError("Side channel does not contain " + keyDesc);
          }
        },
        "delete": function(key) {
          return !!$channelData && $channelData["delete"](key);
        },
        get: function(key) {
          return $channelData && $channelData.get(key);
        },
        has: function(key) {
          return !!$channelData && $channelData.has(key);
        },
        set: function(key, value) {
          if (!$channelData) {
            $channelData = makeChannel();
          }
          $channelData.set(key, value);
        }
      };
      return channel;
    };
  }
});

// node_modules/qs/lib/formats.js
var require_formats = __commonJS({
  "node_modules/qs/lib/formats.js"(exports, module) {
    "use strict";
    var replace = String.prototype.replace;
    var percentTwenties = /%20/g;
    var Format = {
      RFC1738: "RFC1738",
      RFC3986: "RFC3986"
    };
    module.exports = {
      "default": Format.RFC3986,
      formatters: {
        RFC1738: function(value) {
          return replace.call(value, percentTwenties, "+");
        },
        RFC3986: function(value) {
          return String(value);
        }
      },
      RFC1738: Format.RFC1738,
      RFC3986: Format.RFC3986
    };
  }
});

// node_modules/qs/lib/utils.js
var require_utils = __commonJS({
  "node_modules/qs/lib/utils.js"(exports, module) {
    "use strict";
    var formats = require_formats();
    var getSideChannel = require_side_channel();
    var defineProperty = require_es_define_property();
    var has = Object.prototype.hasOwnProperty;
    var isArray = Array.isArray;
    var overflowChannel = getSideChannel();
    var markOverflow = function markOverflow2(obj, maxIndex) {
      overflowChannel.set(obj, maxIndex);
      return obj;
    };
    var isOverflow = function isOverflow2(obj) {
      return overflowChannel.has(obj);
    };
    var getMaxIndex = function getMaxIndex2(obj) {
      return overflowChannel.get(obj);
    };
    var setMaxIndex = function setMaxIndex2(obj, maxIndex) {
      overflowChannel.set(obj, maxIndex);
    };
    var hexTable = function() {
      var array = [];
      for (var i = 0; i < 256; ++i) {
        array[array.length] = "%" + ((i < 16 ? "0" : "") + i.toString(16)).toUpperCase();
      }
      return array;
    }();
    var compactQueue = function compactQueue2(queue) {
      while (queue.length > 1) {
        var item = queue.pop();
        var obj = item.obj[item.prop];
        if (isArray(obj)) {
          var compacted = [];
          for (var j = 0; j < obj.length; ++j) {
            if (typeof obj[j] !== "undefined") {
              compacted[compacted.length] = obj[j];
            }
          }
          item.obj[item.prop] = compacted;
        }
      }
    };
    var arrayToObject = function arrayToObject2(source, options) {
      var obj = options && options.plainObjects ? { __proto__: null } : {};
      for (var i = 0; i < source.length; ++i) {
        if (typeof source[i] !== "undefined") {
          obj[i] = source[i];
        }
      }
      return obj;
    };
    var setProperty = function setProperty2(obj, key, value) {
      if (key === "__proto__" && defineProperty) {
        defineProperty(obj, key, {
          configurable: true,
          enumerable: true,
          value,
          writable: true
        });
      } else {
        obj[key] = value;
      }
    };
    var merge = function merge2(target, source, options) {
      if (!source) {
        return target;
      }
      if (typeof source !== "object" && typeof source !== "function") {
        if (isArray(target)) {
          var nextIndex = target.length;
          if (options && typeof options.arrayLimit === "number" && nextIndex >= options.arrayLimit) {
            if (options.throwOnLimitExceeded) {
              throw new RangeError("Array limit exceeded. Only " + options.arrayLimit + " element" + (options.arrayLimit === 1 ? "" : "s") + " allowed in an array.");
            }
            return markOverflow(arrayToObject(target.concat(source), options), nextIndex);
          }
          target[nextIndex] = source;
        } else if (target && typeof target === "object") {
          if (isOverflow(target)) {
            var newIndex = getMaxIndex(target) + 1;
            target[newIndex] = source;
            setMaxIndex(target, newIndex);
          } else if (options && options.strictMerge) {
            return [target, source];
          } else if (options && (options.plainObjects || options.allowPrototypes) || !has.call(Object.prototype, source)) {
            target[source] = true;
          }
        } else {
          return [target, source];
        }
        return target;
      }
      if (!target || typeof target !== "object") {
        if (isOverflow(source)) {
          var sourceKeys = Object.keys(source);
          var result = options && options.plainObjects ? { __proto__: null, 0: target } : { 0: target };
          for (var m = 0; m < sourceKeys.length; m++) {
            var oldKey = parseInt(sourceKeys[m], 10);
            result[oldKey + 1] = source[sourceKeys[m]];
          }
          return markOverflow(result, getMaxIndex(source) + 1);
        }
        var combined = [target].concat(source);
        if (options && typeof options.arrayLimit === "number" && combined.length > options.arrayLimit) {
          if (options.throwOnLimitExceeded) {
            throw new RangeError("Array limit exceeded. Only " + options.arrayLimit + " element" + (options.arrayLimit === 1 ? "" : "s") + " allowed in an array.");
          }
          return markOverflow(arrayToObject(combined, options), combined.length - 1);
        }
        return combined;
      }
      var mergeTarget = target;
      if (isArray(target) && !isArray(source)) {
        mergeTarget = arrayToObject(target, options);
      }
      if (isArray(target) && isArray(source)) {
        source.forEach(function(item, i) {
          if (has.call(target, i)) {
            var targetItem = target[i];
            if (targetItem && typeof targetItem === "object" && item && typeof item === "object") {
              target[i] = merge2(targetItem, item, options);
            } else {
              target[target.length] = item;
            }
          } else {
            target[i] = item;
          }
        });
        if (options && typeof options.arrayLimit === "number" && target.length > options.arrayLimit) {
          if (options.throwOnLimitExceeded) {
            throw new RangeError("Array limit exceeded. Only " + options.arrayLimit + " element" + (options.arrayLimit === 1 ? "" : "s") + " allowed in an array.");
          }
          return markOverflow(arrayToObject(target, options), target.length - 1);
        }
        return target;
      }
      return Object.keys(source).reduce(function(acc, key) {
        var value = source[key];
        if (has.call(acc, key)) {
          setProperty(acc, key, merge2(acc[key], value, options));
        } else {
          setProperty(acc, key, value);
        }
        if (isOverflow(source) && !isOverflow(acc)) {
          markOverflow(acc, getMaxIndex(source));
        }
        if (isOverflow(acc)) {
          var keyNum = parseInt(key, 10);
          if (String(keyNum) === key && keyNum >= 0 && keyNum > getMaxIndex(acc)) {
            setMaxIndex(acc, keyNum);
          }
        }
        return acc;
      }, mergeTarget);
    };
    var assign = function assignSingleSource(target, source) {
      return Object.keys(source).reduce(function(acc, key) {
        setProperty(acc, key, source[key]);
        return acc;
      }, target);
    };
    var decode = function(str3, defaultDecoder, charset) {
      var strWithoutPlus = str3.replace(/\+/g, " ");
      if (charset === "iso-8859-1") {
        return strWithoutPlus.replace(/%[0-9a-f]{2}/gi, unescape);
      }
      try {
        return decodeURIComponent(strWithoutPlus);
      } catch (e) {
        return strWithoutPlus;
      }
    };
    var limit = 1024;
    var encode = function encode2(str3, defaultEncoder, charset, kind, format2) {
      if (str3.length === 0) {
        return str3;
      }
      var string = str3;
      if (typeof str3 === "symbol") {
        string = Symbol.prototype.toString.call(str3);
      } else if (typeof str3 !== "string") {
        string = String(str3);
      }
      if (charset === "iso-8859-1") {
        return escape(string).replace(/%u[0-9a-f]{4}/gi, function($0) {
          return "%26%23" + parseInt($0.slice(2), 16) + "%3B";
        });
      }
      var out = "";
      for (var j = 0; j < string.length; j += limit) {
        var segment = string.length >= limit ? string.slice(j, j + limit) : string;
        if (j + limit < string.length) {
          var last = segment.charCodeAt(segment.length - 1);
          if (last >= 55296 && last <= 56319) {
            segment = segment.slice(0, -1);
            j -= 1;
          }
        }
        var arr = [];
        for (var i = 0; i < segment.length; ++i) {
          var c2 = segment.charCodeAt(i);
          if (c2 === 45 || c2 === 46 || c2 === 95 || c2 === 126 || c2 >= 48 && c2 <= 57 || c2 >= 65 && c2 <= 90 || c2 >= 97 && c2 <= 122 || format2 === formats.RFC1738 && (c2 === 40 || c2 === 41)) {
            arr[arr.length] = segment.charAt(i);
            continue;
          }
          if (c2 < 128) {
            arr[arr.length] = hexTable[c2];
            continue;
          }
          if (c2 < 2048) {
            arr[arr.length] = hexTable[192 | c2 >> 6] + hexTable[128 | c2 & 63];
            continue;
          }
          if (c2 < 55296 || c2 >= 57344) {
            arr[arr.length] = hexTable[224 | c2 >> 12] + hexTable[128 | c2 >> 6 & 63] + hexTable[128 | c2 & 63];
            continue;
          }
          i += 1;
          c2 = 65536 + ((c2 & 1023) << 10 | segment.charCodeAt(i) & 1023);
          arr[arr.length] = hexTable[240 | c2 >> 18] + hexTable[128 | c2 >> 12 & 63] + hexTable[128 | c2 >> 6 & 63] + hexTable[128 | c2 & 63];
        }
        out += arr.join("");
      }
      return out;
    };
    var compact = function compact2(value) {
      var queue = [{ obj: { o: value }, prop: "o" }];
      var refs = getSideChannel();
      for (var i = 0; i < queue.length; ++i) {
        var item = queue[i];
        var obj = item.obj[item.prop];
        var keys = Object.keys(obj);
        for (var j = 0; j < keys.length; ++j) {
          var key = keys[j];
          var val = obj[key];
          if (typeof val === "object" && val !== null && !refs.has(val)) {
            queue[queue.length] = { obj, prop: key };
            refs.set(val, true);
          }
        }
      }
      compactQueue(queue);
      return value;
    };
    var isRegExp = function isRegExp2(obj) {
      return Object.prototype.toString.call(obj) === "[object RegExp]";
    };
    var isBuffer = function isBuffer2(obj) {
      if (!obj || typeof obj !== "object") {
        return false;
      }
      return !!(obj.constructor && typeof obj.constructor.isBuffer === "function" && obj.constructor.isBuffer(obj));
    };
    var combine = function combine2(a, b, arrayLimit, plainObjects, throwOnLimitExceeded) {
      if (isOverflow(a)) {
        if (throwOnLimitExceeded) {
          throw new RangeError("Array limit exceeded. Only " + arrayLimit + " element" + (arrayLimit === 1 ? "" : "s") + " allowed in an array.");
        }
        var bValues = isArray(b) ? b : [b];
        var newIndex = getMaxIndex(a);
        for (var i = 0; i < bValues.length; ++i) {
          newIndex += 1;
          a[newIndex] = bValues[i];
        }
        setMaxIndex(a, newIndex);
        return a;
      }
      var result = [].concat(a, b);
      if (result.length > arrayLimit) {
        if (throwOnLimitExceeded) {
          throw new RangeError("Array limit exceeded. Only " + arrayLimit + " element" + (arrayLimit === 1 ? "" : "s") + " allowed in an array.");
        }
        return markOverflow(arrayToObject(result, { plainObjects }), result.length - 1);
      }
      return result;
    };
    var maybeMap = function maybeMap2(val, fn) {
      if (isArray(val)) {
        var mapped = [];
        for (var i = 0; i < val.length; i += 1) {
          mapped[mapped.length] = fn(val[i]);
        }
        return mapped;
      }
      return fn(val);
    };
    module.exports = {
      arrayToObject,
      assign,
      combine,
      compact,
      decode,
      encode,
      isBuffer,
      isOverflow,
      isRegExp,
      markOverflow,
      maybeMap,
      merge
    };
  }
});

// node_modules/qs/lib/stringify.js
var require_stringify = __commonJS({
  "node_modules/qs/lib/stringify.js"(exports, module) {
    "use strict";
    var getSideChannel = require_side_channel();
    var utils = require_utils();
    var formats = require_formats();
    var has = Object.prototype.hasOwnProperty;
    var arrayPrefixGenerators = {
      brackets: function brackets(prefix) {
        return prefix + "[]";
      },
      comma: "comma",
      indices: function indices(prefix, key) {
        return prefix + "[" + key + "]";
      },
      repeat: function repeat(prefix) {
        return prefix;
      }
    };
    var isArray = Array.isArray;
    var push = Array.prototype.push;
    var pushToArray = function(arr, valueOrArray) {
      push.apply(arr, isArray(valueOrArray) ? valueOrArray : [valueOrArray]);
    };
    var toISO = Date.prototype.toISOString;
    var defaultFormat = formats["default"];
    var defaults = {
      addQueryPrefix: false,
      allowDots: false,
      allowEmptyArrays: false,
      arrayFormat: "indices",
      charset: "utf-8",
      charsetSentinel: false,
      commaRoundTrip: false,
      delimiter: "&",
      depth: Infinity,
      encode: true,
      encodeDotInKeys: false,
      encoder: utils.encode,
      encodeValuesOnly: false,
      filter: void 0,
      format: defaultFormat,
      formatter: formats.formatters[defaultFormat],
      // deprecated
      indices: false,
      serializeDate: function serializeDate(date) {
        return toISO.call(date);
      },
      skipNulls: false,
      strictNullHandling: false
    };
    var isNonNullishPrimitive = function isNonNullishPrimitive2(v) {
      return typeof v === "string" || typeof v === "number" || typeof v === "boolean" || typeof v === "symbol" || typeof v === "bigint";
    };
    var sentinel = {};
    var stringify2 = function stringify3(object, prefix, generateArrayPrefix, commaRoundTrip, allowEmptyArrays, strictNullHandling, skipNulls, encodeDotInKeys, encoder, filter, sort, allowDots, serializeDate, format2, formatter, encodeValuesOnly, charset, sideChannel, depth, currentDepth) {
      var obj = object;
      if (currentDepth > depth) {
        throw new RangeError("Input depth exceeded depth option of " + depth);
      }
      var tmpSc = sideChannel;
      var step = 0;
      var findFlag = false;
      while ((tmpSc = tmpSc.get(sentinel)) !== void 0 && !findFlag) {
        var pos = tmpSc.get(object);
        step += 1;
        if (typeof pos !== "undefined") {
          if (pos === step) {
            throw new RangeError("Cyclic object value");
          } else {
            findFlag = true;
          }
        }
        if (typeof tmpSc.get(sentinel) === "undefined") {
          step = 0;
        }
      }
      obj = typeof filter === "function" ? filter(prefix, obj) : obj;
      if (obj instanceof Date) {
        obj = serializeDate(obj);
      } else if (generateArrayPrefix === "comma" && isArray(obj)) {
        obj = utils.maybeMap(obj, function(value2) {
          if (value2 instanceof Date) {
            return serializeDate(value2);
          }
          return value2;
        });
      }
      if (obj === null) {
        if (strictNullHandling) {
          return formatter(encoder && !encodeValuesOnly ? encoder(prefix, defaults.encoder, charset, "key", format2) : prefix);
        }
        obj = "";
      }
      if (isNonNullishPrimitive(obj) || utils.isBuffer(obj)) {
        if (encoder) {
          var keyValue = encodeValuesOnly ? prefix : encoder(prefix, defaults.encoder, charset, "key", format2);
          return [formatter(keyValue) + "=" + formatter(encoder(obj, defaults.encoder, charset, "value", format2))];
        }
        return [formatter(prefix) + "=" + formatter(String(obj))];
      }
      var values = [];
      if (typeof obj === "undefined") {
        return values;
      }
      var objKeys;
      if (generateArrayPrefix === "comma" && isArray(obj)) {
        if (encodeValuesOnly && encoder) {
          obj = utils.maybeMap(obj, function(v) {
            return v == null ? v : encoder(v);
          });
        }
        objKeys = [{ value: obj.length > 0 ? obj.join(",") || null : void 0 }];
      } else if (isArray(filter)) {
        objKeys = filter;
      } else {
        var keys = Object.keys(obj);
        objKeys = sort ? keys.sort(sort) : keys;
      }
      var encodedPrefix = encodeDotInKeys ? String(prefix).replace(/\./g, "%2E") : String(prefix);
      var adjustedPrefix = commaRoundTrip && isArray(obj) && obj.length === 1 ? encodedPrefix + "[]" : encodedPrefix;
      if (allowEmptyArrays && isArray(obj) && obj.length === 0 && Object.keys(obj).length === 0) {
        return adjustedPrefix + "[]";
      }
      for (var j = 0; j < objKeys.length; ++j) {
        var key = objKeys[j];
        var value = typeof key === "object" && key && typeof key.value !== "undefined" ? key.value : obj[key];
        if (skipNulls && value === null) {
          continue;
        }
        var encodedKey = allowDots && encodeDotInKeys ? String(key).replace(/\./g, "%2E") : String(key);
        var keyPrefix = isArray(obj) ? typeof generateArrayPrefix === "function" ? generateArrayPrefix(adjustedPrefix, encodedKey) : adjustedPrefix : adjustedPrefix + (allowDots ? "." + encodedKey : "[" + encodedKey + "]");
        sideChannel.set(object, step);
        var valueSideChannel = getSideChannel();
        valueSideChannel.set(sentinel, sideChannel);
        pushToArray(values, stringify3(
          value,
          keyPrefix,
          generateArrayPrefix,
          commaRoundTrip,
          allowEmptyArrays,
          strictNullHandling,
          skipNulls,
          encodeDotInKeys,
          generateArrayPrefix === "comma" && encodeValuesOnly && isArray(obj) ? null : encoder,
          filter,
          sort,
          allowDots,
          serializeDate,
          format2,
          formatter,
          encodeValuesOnly,
          charset,
          valueSideChannel,
          depth,
          currentDepth + 1
        ));
      }
      return values;
    };
    var normalizeStringifyOptions = function normalizeStringifyOptions2(opts) {
      if (!opts) {
        return defaults;
      }
      if (typeof opts.allowEmptyArrays !== "undefined" && typeof opts.allowEmptyArrays !== "boolean") {
        throw new TypeError("`allowEmptyArrays` option can only be `true` or `false`, when provided");
      }
      if (typeof opts.encodeDotInKeys !== "undefined" && typeof opts.encodeDotInKeys !== "boolean") {
        throw new TypeError("`encodeDotInKeys` option can only be `true` or `false`, when provided");
      }
      if (opts.encoder !== null && typeof opts.encoder !== "undefined" && typeof opts.encoder !== "function") {
        throw new TypeError("Encoder has to be a function.");
      }
      var charset = opts.charset || defaults.charset;
      if (typeof opts.charset !== "undefined" && opts.charset !== "utf-8" && opts.charset !== "iso-8859-1") {
        throw new TypeError("The charset option must be either utf-8, iso-8859-1, or undefined");
      }
      var format2 = formats["default"];
      if (typeof opts.format !== "undefined") {
        if (!has.call(formats.formatters, opts.format)) {
          throw new TypeError("Unknown format option provided.");
        }
        format2 = opts.format;
      }
      var formatter = formats.formatters[format2];
      var filter = defaults.filter;
      if (typeof opts.filter === "function" || isArray(opts.filter)) {
        filter = opts.filter;
      }
      var arrayFormat;
      if (opts.arrayFormat in arrayPrefixGenerators) {
        arrayFormat = opts.arrayFormat;
      } else if ("indices" in opts) {
        arrayFormat = opts.indices ? "indices" : "repeat";
      } else {
        arrayFormat = defaults.arrayFormat;
      }
      if ("commaRoundTrip" in opts && typeof opts.commaRoundTrip !== "boolean") {
        throw new TypeError("`commaRoundTrip` must be a boolean, or absent");
      }
      var allowDots = typeof opts.allowDots === "undefined" ? opts.encodeDotInKeys === true ? true : defaults.allowDots : !!opts.allowDots;
      return {
        addQueryPrefix: typeof opts.addQueryPrefix === "boolean" ? opts.addQueryPrefix : defaults.addQueryPrefix,
        allowDots,
        allowEmptyArrays: typeof opts.allowEmptyArrays === "boolean" ? !!opts.allowEmptyArrays : defaults.allowEmptyArrays,
        arrayFormat,
        charset,
        charsetSentinel: typeof opts.charsetSentinel === "boolean" ? opts.charsetSentinel : defaults.charsetSentinel,
        commaRoundTrip: !!opts.commaRoundTrip,
        delimiter: typeof opts.delimiter === "undefined" ? defaults.delimiter : opts.delimiter,
        depth: typeof opts.depth === "number" ? opts.depth : defaults.depth,
        encode: typeof opts.encode === "boolean" ? opts.encode : defaults.encode,
        encodeDotInKeys: typeof opts.encodeDotInKeys === "boolean" ? opts.encodeDotInKeys : defaults.encodeDotInKeys,
        encoder: typeof opts.encoder === "function" ? opts.encoder : defaults.encoder,
        encodeValuesOnly: typeof opts.encodeValuesOnly === "boolean" ? opts.encodeValuesOnly : defaults.encodeValuesOnly,
        filter,
        format: format2,
        formatter,
        serializeDate: typeof opts.serializeDate === "function" ? opts.serializeDate : defaults.serializeDate,
        skipNulls: typeof opts.skipNulls === "boolean" ? opts.skipNulls : defaults.skipNulls,
        sort: typeof opts.sort === "function" ? opts.sort : null,
        strictNullHandling: typeof opts.strictNullHandling === "boolean" ? opts.strictNullHandling : defaults.strictNullHandling
      };
    };
    module.exports = function(object, opts) {
      var obj = object;
      var options = normalizeStringifyOptions(opts);
      var objKeys;
      var filter;
      if (typeof options.filter === "function") {
        filter = options.filter;
        obj = filter("", obj);
      } else if (isArray(options.filter)) {
        filter = options.filter;
        objKeys = filter;
      }
      var keys = [];
      if (typeof obj !== "object" || obj === null) {
        return "";
      }
      var generateArrayPrefix = arrayPrefixGenerators[options.arrayFormat];
      var commaRoundTrip = generateArrayPrefix === "comma" && options.commaRoundTrip;
      if (!objKeys) {
        objKeys = Object.keys(obj);
      }
      if (options.sort) {
        objKeys.sort(options.sort);
      }
      var sideChannel = getSideChannel();
      for (var i = 0; i < objKeys.length; ++i) {
        var key = objKeys[i];
        if (typeof key === "undefined" || key === null) {
          continue;
        }
        var value = obj[key];
        if (options.skipNulls && value === null) {
          continue;
        }
        var encodedKey = options.encodeDotInKeys ? String(key).replace(/\./g, "%2E") : String(key);
        pushToArray(keys, stringify2(
          value,
          encodedKey,
          generateArrayPrefix,
          commaRoundTrip,
          options.allowEmptyArrays,
          options.strictNullHandling,
          options.skipNulls,
          options.encodeDotInKeys,
          options.encode ? options.encoder : null,
          options.filter,
          options.sort,
          options.allowDots,
          options.serializeDate,
          options.format,
          options.formatter,
          options.encodeValuesOnly,
          options.charset,
          sideChannel,
          options.depth,
          0
        ));
      }
      var joined = keys.join(options.delimiter);
      var prefix = options.addQueryPrefix === true ? "?" : "";
      if (options.charsetSentinel) {
        if (options.charset === "iso-8859-1") {
          prefix += "utf8=%26%2310003%3B" + options.delimiter;
        } else {
          prefix += "utf8=%E2%9C%93" + options.delimiter;
        }
      }
      return joined.length > 0 ? prefix + joined : "";
    };
  }
});

// node_modules/qs/lib/parse.js
var require_parse = __commonJS({
  "node_modules/qs/lib/parse.js"(exports, module) {
    "use strict";
    var utils = require_utils();
    var has = Object.prototype.hasOwnProperty;
    var isArray = Array.isArray;
    var defaults = {
      allowDots: false,
      allowEmptyArrays: false,
      allowPrototypes: false,
      allowSparse: false,
      arrayLimit: 20,
      charset: "utf-8",
      charsetSentinel: false,
      comma: false,
      decodeDotInKeys: false,
      decoder: utils.decode,
      delimiter: "&",
      depth: 5,
      duplicates: "combine",
      ignoreQueryPrefix: false,
      interpretNumericEntities: false,
      parameterLimit: 1e3,
      parseArrays: true,
      plainObjects: false,
      strictDepth: false,
      strictMerge: true,
      strictNullHandling: false,
      throwOnLimitExceeded: false
    };
    var interpretNumericEntities = function(str3) {
      return str3.replace(/&#(\d+);/g, function($0, numberStr) {
        return String.fromCharCode(parseInt(numberStr, 10));
      });
    };
    var parseArrayValue = function(val, options, currentArrayLength) {
      if (val && typeof val === "string" && options.comma && val.indexOf(",") > -1) {
        if (options.throwOnLimitExceeded) {
          var commaCount = 0;
          var commaIndex = val.indexOf(",");
          while (commaIndex > -1) {
            commaCount += 1;
            if (commaCount >= options.arrayLimit) {
              throw new RangeError("Array limit exceeded. Only " + options.arrayLimit + " element" + (options.arrayLimit === 1 ? "" : "s") + " allowed in an array.");
            }
            commaIndex = val.indexOf(",", commaIndex + 1);
          }
        }
        return val.split(",");
      }
      if (options.throwOnLimitExceeded && currentArrayLength >= options.arrayLimit) {
        throw new RangeError("Array limit exceeded. Only " + options.arrayLimit + " element" + (options.arrayLimit === 1 ? "" : "s") + " allowed in an array.");
      }
      return val;
    };
    var isoSentinel = "utf8=%26%2310003%3B";
    var charsetSentinel = "utf8=%E2%9C%93";
    var parseValues2 = function parseQueryStringValues(str3, options) {
      var obj = { __proto__: null };
      var cleanStr = options.ignoreQueryPrefix ? str3.replace(/^\?/, "") : str3;
      cleanStr = cleanStr.replace(/%5B/gi, "[").replace(/%5D/gi, "]");
      var limit = options.parameterLimit === Infinity ? void 0 : options.parameterLimit;
      var parts = cleanStr.split(
        options.delimiter,
        options.throwOnLimitExceeded && typeof limit !== "undefined" ? limit + 1 : limit
      );
      if (options.throwOnLimitExceeded && typeof limit !== "undefined" && parts.length > limit) {
        throw new RangeError("Parameter limit exceeded. Only " + limit + " parameter" + (limit === 1 ? "" : "s") + " allowed.");
      }
      var skipIndex = -1;
      var i;
      var charset = options.charset;
      if (options.charsetSentinel) {
        for (i = 0; i < parts.length; ++i) {
          if (parts[i].indexOf("utf8=") === 0) {
            if (parts[i] === charsetSentinel) {
              charset = "utf-8";
            } else if (parts[i] === isoSentinel) {
              charset = "iso-8859-1";
            }
            skipIndex = i;
            i = parts.length;
          }
        }
      }
      for (i = 0; i < parts.length; ++i) {
        if (i === skipIndex) {
          continue;
        }
        var part = parts[i];
        var bracketEqualsPos = part.indexOf("]=");
        var pos = bracketEqualsPos === -1 ? part.indexOf("=") : bracketEqualsPos + 1;
        var key;
        var val;
        if (pos === -1) {
          key = options.decoder(part, defaults.decoder, charset, "key");
          val = options.strictNullHandling ? null : "";
        } else {
          key = options.decoder(part.slice(0, pos), defaults.decoder, charset, "key");
          if (key !== null) {
            val = utils.maybeMap(
              parseArrayValue(
                part.slice(pos + 1),
                options,
                isArray(obj[key]) ? obj[key].length : 0
              ),
              function(encodedVal) {
                return options.decoder(encodedVal, defaults.decoder, charset, "value");
              }
            );
          }
        }
        if (val && options.interpretNumericEntities && charset === "iso-8859-1") {
          val = interpretNumericEntities(String(val));
        }
        if (part.indexOf("[]=") > -1) {
          val = isArray(val) ? [val] : val;
        }
        if (options.comma && isArray(val) && val.length > options.arrayLimit) {
          val = utils.combine([], val, options.arrayLimit, options.plainObjects, options.throwOnLimitExceeded);
        }
        if (key !== null) {
          var existing = has.call(obj, key);
          if (existing && (options.duplicates === "combine" || part.indexOf("[]=") > -1)) {
            obj[key] = utils.combine(
              obj[key],
              val,
              options.arrayLimit,
              options.plainObjects,
              options.throwOnLimitExceeded
            );
          } else if (!existing || options.duplicates === "last") {
            obj[key] = val;
          }
        }
      }
      return obj;
    };
    var parseObject = function(chain, val, options, valuesParsed) {
      var currentArrayLength = 0;
      if (chain.length > 0 && chain[chain.length - 1] === "[]") {
        var parentKey = chain.slice(0, -1).join("");
        currentArrayLength = Array.isArray(val) && val[parentKey] ? val[parentKey].length : 0;
      }
      var leaf = valuesParsed ? val : parseArrayValue(val, options, currentArrayLength);
      for (var i = chain.length - 1; i >= 0; --i) {
        var obj;
        var root = chain[i];
        if (root === "[]" && options.parseArrays) {
          if (utils.isOverflow(leaf)) {
            obj = leaf;
          } else {
            obj = options.allowEmptyArrays && (leaf === "" || options.strictNullHandling && leaf === null) ? [] : utils.combine(
              [],
              leaf,
              options.arrayLimit,
              options.plainObjects,
              options.throwOnLimitExceeded
            );
          }
        } else {
          obj = options.plainObjects ? { __proto__: null } : {};
          var cleanRoot = root.charAt(0) === "[" && root.charAt(root.length - 1) === "]" ? root.slice(1, -1) : root;
          var decodedRoot = options.decodeDotInKeys ? cleanRoot.replace(/%2E/g, ".") : cleanRoot;
          var index = parseInt(decodedRoot, 10);
          var isValidArrayIndex = !isNaN(index) && root !== decodedRoot && String(index) === decodedRoot && index >= 0 && options.parseArrays;
          if (!options.parseArrays && decodedRoot === "") {
            obj = { 0: leaf };
          } else if (isValidArrayIndex && index < options.arrayLimit) {
            obj = [];
            obj[index] = leaf;
          } else if (isValidArrayIndex && options.throwOnLimitExceeded) {
            throw new RangeError("Array limit exceeded. Only " + options.arrayLimit + " element" + (options.arrayLimit === 1 ? "" : "s") + " allowed in an array.");
          } else if (isValidArrayIndex) {
            obj[index] = leaf;
            utils.markOverflow(obj, index);
          } else if (decodedRoot !== "__proto__") {
            obj[decodedRoot] = leaf;
          }
        }
        leaf = obj;
      }
      return leaf;
    };
    var splitKeyIntoSegments = function splitKeyIntoSegments2(originalKey, options) {
      var key = options.allowDots ? originalKey.replace(/\.([^.[]+)/g, "[$1]") : originalKey;
      if (options.depth <= 0) {
        if (!options.plainObjects && has.call(Object.prototype, key)) {
          if (!options.allowPrototypes) {
            return;
          }
        }
        return [key];
      }
      var segments = [];
      var first = key.indexOf("[");
      var parent = first >= 0 ? key.slice(0, first) : key;
      if (parent) {
        if (!options.plainObjects && has.call(Object.prototype, parent)) {
          if (!options.allowPrototypes) {
            return;
          }
        }
        segments[segments.length] = parent;
      }
      var n = key.length;
      var open = first;
      var collected = 0;
      while (open >= 0 && collected < options.depth) {
        var level = 1;
        var i = open + 1;
        var close = -1;
        while (i < n && close < 0) {
          var cu = key.charCodeAt(i);
          if (cu === 91) {
            level += 1;
          } else if (cu === 93) {
            level -= 1;
            if (level === 0) {
              close = i;
            }
          }
          i += 1;
        }
        if (close < 0) {
          segments[segments.length] = "[" + key.slice(open) + "]";
          return segments;
        }
        var seg = key.slice(open, close + 1);
        var content = seg.slice(1, -1);
        if (!options.plainObjects && has.call(Object.prototype, content) && !options.allowPrototypes) {
          return;
        }
        segments[segments.length] = seg;
        collected += 1;
        open = key.indexOf("[", close + 1);
      }
      if (open >= 0) {
        if (options.strictDepth === true) {
          throw new RangeError("Input depth exceeded depth option of " + options.depth + " and strictDepth is true");
        }
        segments[segments.length] = "[" + key.slice(open) + "]";
      }
      return segments;
    };
    var parseKeys = function parseQueryStringKeys(givenKey, val, options, valuesParsed) {
      if (!givenKey) {
        return;
      }
      var keys = splitKeyIntoSegments(givenKey, options);
      if (!keys) {
        return;
      }
      return parseObject(keys, val, options, valuesParsed);
    };
    var normalizeParseOptions = function normalizeParseOptions2(opts) {
      if (!opts) {
        return defaults;
      }
      if (typeof opts.allowEmptyArrays !== "undefined" && typeof opts.allowEmptyArrays !== "boolean") {
        throw new TypeError("`allowEmptyArrays` option can only be `true` or `false`, when provided");
      }
      if (typeof opts.decodeDotInKeys !== "undefined" && typeof opts.decodeDotInKeys !== "boolean") {
        throw new TypeError("`decodeDotInKeys` option can only be `true` or `false`, when provided");
      }
      if (opts.decoder !== null && typeof opts.decoder !== "undefined" && typeof opts.decoder !== "function") {
        throw new TypeError("Decoder has to be a function.");
      }
      if (typeof opts.charset !== "undefined" && opts.charset !== "utf-8" && opts.charset !== "iso-8859-1") {
        throw new TypeError("The charset option must be either utf-8, iso-8859-1, or undefined");
      }
      if (typeof opts.throwOnLimitExceeded !== "undefined" && typeof opts.throwOnLimitExceeded !== "boolean") {
        throw new TypeError("`throwOnLimitExceeded` option must be a boolean");
      }
      var charset = typeof opts.charset === "undefined" ? defaults.charset : opts.charset;
      var duplicates = typeof opts.duplicates === "undefined" ? defaults.duplicates : opts.duplicates;
      if (duplicates !== "combine" && duplicates !== "first" && duplicates !== "last") {
        throw new TypeError("The duplicates option must be either combine, first, or last");
      }
      var allowDots = typeof opts.allowDots === "undefined" ? opts.decodeDotInKeys === true ? true : defaults.allowDots : !!opts.allowDots;
      return {
        allowDots,
        allowEmptyArrays: typeof opts.allowEmptyArrays === "boolean" ? !!opts.allowEmptyArrays : defaults.allowEmptyArrays,
        allowPrototypes: typeof opts.allowPrototypes === "boolean" ? opts.allowPrototypes : defaults.allowPrototypes,
        allowSparse: typeof opts.allowSparse === "boolean" ? opts.allowSparse : defaults.allowSparse,
        arrayLimit: typeof opts.arrayLimit === "number" ? opts.arrayLimit : defaults.arrayLimit,
        charset,
        charsetSentinel: typeof opts.charsetSentinel === "boolean" ? opts.charsetSentinel : defaults.charsetSentinel,
        comma: typeof opts.comma === "boolean" ? opts.comma : defaults.comma,
        decodeDotInKeys: typeof opts.decodeDotInKeys === "boolean" ? opts.decodeDotInKeys : defaults.decodeDotInKeys,
        decoder: typeof opts.decoder === "function" ? opts.decoder : defaults.decoder,
        delimiter: typeof opts.delimiter === "string" || utils.isRegExp(opts.delimiter) ? opts.delimiter : defaults.delimiter,
        // eslint-disable-next-line no-implicit-coercion, no-extra-parens
        depth: typeof opts.depth === "number" || opts.depth === false ? +opts.depth : defaults.depth,
        duplicates,
        ignoreQueryPrefix: opts.ignoreQueryPrefix === true,
        interpretNumericEntities: typeof opts.interpretNumericEntities === "boolean" ? opts.interpretNumericEntities : defaults.interpretNumericEntities,
        parameterLimit: typeof opts.parameterLimit === "number" ? opts.parameterLimit : defaults.parameterLimit,
        parseArrays: opts.parseArrays !== false,
        plainObjects: typeof opts.plainObjects === "boolean" ? opts.plainObjects : defaults.plainObjects,
        strictDepth: typeof opts.strictDepth === "boolean" ? !!opts.strictDepth : defaults.strictDepth,
        strictMerge: typeof opts.strictMerge === "boolean" ? !!opts.strictMerge : defaults.strictMerge,
        strictNullHandling: typeof opts.strictNullHandling === "boolean" ? opts.strictNullHandling : defaults.strictNullHandling,
        throwOnLimitExceeded: typeof opts.throwOnLimitExceeded === "boolean" ? opts.throwOnLimitExceeded : false
      };
    };
    module.exports = function(str3, opts) {
      var options = normalizeParseOptions(opts);
      if (str3 === "" || str3 === null || typeof str3 === "undefined") {
        return options.plainObjects ? { __proto__: null } : {};
      }
      var tempObj = typeof str3 === "string" ? parseValues2(str3, options) : str3;
      var obj = options.plainObjects ? { __proto__: null } : {};
      var keys = Object.keys(tempObj);
      for (var i = 0; i < keys.length; ++i) {
        var key = keys[i];
        var newObj = parseKeys(key, tempObj[key], options, typeof str3 === "string");
        obj = utils.merge(obj, newObj, options);
      }
      if (options.allowSparse === true) {
        return obj;
      }
      return utils.compact(obj);
    };
  }
});

// node_modules/qs/lib/index.js
var require_lib2 = __commonJS({
  "node_modules/qs/lib/index.js"(exports, module) {
    "use strict";
    var stringify2 = require_stringify();
    var parse2 = require_parse();
    var formats = require_formats();
    module.exports = {
      formats,
      parse: parse2,
      stringify: stringify2
    };
  }
});

// node_modules/url/url.js
var require_url = __commonJS({
  "node_modules/url/url.js"(exports) {
    "use strict";
    var punycode = require_punycode();
    function Url() {
      this.protocol = null;
      this.slashes = null;
      this.auth = null;
      this.host = null;
      this.port = null;
      this.hostname = null;
      this.hash = null;
      this.search = null;
      this.query = null;
      this.pathname = null;
      this.path = null;
      this.href = null;
    }
    var protocolPattern = /^([a-z0-9.+-]+:)/i;
    var portPattern = /:[0-9]*$/;
    var simplePathPattern = /^(\/\/?(?!\/)[^?\s]*)(\?[^\s]*)?$/;
    var delims = [
      "<",
      ">",
      '"',
      "`",
      " ",
      "\r",
      "\n",
      "	"
    ];
    var unwise = [
      "{",
      "}",
      "|",
      "\\",
      "^",
      "`"
    ].concat(delims);
    var autoEscape = ["'"].concat(unwise);
    var nonHostChars = [
      "%",
      "/",
      "?",
      ";",
      "#"
    ].concat(autoEscape);
    var hostEndingChars = [
      "/",
      "?",
      "#"
    ];
    var hostnameMaxLen = 255;
    var hostnamePartPattern = /^[+a-z0-9A-Z_-]{0,63}$/;
    var hostnamePartStart = /^([+a-z0-9A-Z_-]{0,63})(.*)$/;
    var unsafeProtocol = {
      javascript: true,
      "javascript:": true
    };
    var hostlessProtocol = {
      javascript: true,
      "javascript:": true
    };
    var slashedProtocol = {
      http: true,
      https: true,
      ftp: true,
      gopher: true,
      file: true,
      "http:": true,
      "https:": true,
      "ftp:": true,
      "gopher:": true,
      "file:": true
    };
    var querystring = require_lib2();
    function urlParse(url, parseQueryString, slashesDenoteHost) {
      if (url && typeof url === "object" && url instanceof Url) {
        return url;
      }
      var u = new Url();
      u.parse(url, parseQueryString, slashesDenoteHost);
      return u;
    }
    Url.prototype.parse = function(url, parseQueryString, slashesDenoteHost) {
      if (typeof url !== "string") {
        throw new TypeError("Parameter 'url' must be a string, not " + typeof url);
      }
      var queryIndex = url.indexOf("?"), splitter = queryIndex !== -1 && queryIndex < url.indexOf("#") ? "?" : "#", uSplit = url.split(splitter), slashRegex = /\\/g;
      uSplit[0] = uSplit[0].replace(slashRegex, "/");
      url = uSplit.join(splitter);
      var rest = url;
      rest = rest.trim();
      if (!slashesDenoteHost && url.split("#").length === 1) {
        var simplePath = simplePathPattern.exec(rest);
        if (simplePath) {
          this.path = rest;
          this.href = rest;
          this.pathname = simplePath[1];
          if (simplePath[2]) {
            this.search = simplePath[2];
            if (parseQueryString) {
              this.query = querystring.parse(this.search.substr(1));
            } else {
              this.query = this.search.substr(1);
            }
          } else if (parseQueryString) {
            this.search = "";
            this.query = {};
          }
          return this;
        }
      }
      var proto = protocolPattern.exec(rest);
      if (proto) {
        proto = proto[0];
        var lowerProto = proto.toLowerCase();
        this.protocol = lowerProto;
        rest = rest.substr(proto.length);
      }
      if (slashesDenoteHost || proto || rest.match(/^\/\/[^@/]+@[^@/]+/)) {
        var slashes = rest.substr(0, 2) === "//";
        if (slashes && !(proto && hostlessProtocol[proto])) {
          rest = rest.substr(2);
          this.slashes = true;
        }
      }
      if (!hostlessProtocol[proto] && (slashes || proto && !slashedProtocol[proto])) {
        var hostEnd = -1;
        for (var i = 0; i < hostEndingChars.length; i++) {
          var hec = rest.indexOf(hostEndingChars[i]);
          if (hec !== -1 && (hostEnd === -1 || hec < hostEnd)) {
            hostEnd = hec;
          }
        }
        var auth, atSign;
        if (hostEnd === -1) {
          atSign = rest.lastIndexOf("@");
        } else {
          atSign = rest.lastIndexOf("@", hostEnd);
        }
        if (atSign !== -1) {
          auth = rest.slice(0, atSign);
          rest = rest.slice(atSign + 1);
          this.auth = decodeURIComponent(auth);
        }
        hostEnd = -1;
        for (var i = 0; i < nonHostChars.length; i++) {
          var hec = rest.indexOf(nonHostChars[i]);
          if (hec !== -1 && (hostEnd === -1 || hec < hostEnd)) {
            hostEnd = hec;
          }
        }
        if (hostEnd === -1) {
          hostEnd = rest.length;
        }
        this.host = rest.slice(0, hostEnd);
        rest = rest.slice(hostEnd);
        this.parseHost();
        this.hostname = this.hostname || "";
        var ipv6Hostname = this.hostname[0] === "[" && this.hostname[this.hostname.length - 1] === "]";
        if (!ipv6Hostname) {
          var hostparts = this.hostname.split(/\./);
          for (var i = 0, l = hostparts.length; i < l; i++) {
            var part = hostparts[i];
            if (!part) {
              continue;
            }
            if (!part.match(hostnamePartPattern)) {
              var newpart = "";
              for (var j = 0, k = part.length; j < k; j++) {
                if (part.charCodeAt(j) > 127) {
                  newpart += "x";
                } else {
                  newpart += part[j];
                }
              }
              if (!newpart.match(hostnamePartPattern)) {
                var validParts = hostparts.slice(0, i);
                var notHost = hostparts.slice(i + 1);
                var bit = part.match(hostnamePartStart);
                if (bit) {
                  validParts.push(bit[1]);
                  notHost.unshift(bit[2]);
                }
                if (notHost.length) {
                  rest = "/" + notHost.join(".") + rest;
                }
                this.hostname = validParts.join(".");
                break;
              }
            }
          }
        }
        if (this.hostname.length > hostnameMaxLen) {
          this.hostname = "";
        } else {
          this.hostname = this.hostname.toLowerCase();
        }
        if (!ipv6Hostname) {
          this.hostname = punycode.toASCII(this.hostname);
        }
        var p = this.port ? ":" + this.port : "";
        var h = this.hostname || "";
        this.host = h + p;
        this.href += this.host;
        if (ipv6Hostname) {
          this.hostname = this.hostname.substr(1, this.hostname.length - 2);
          if (rest[0] !== "/") {
            rest = "/" + rest;
          }
        }
      }
      if (!unsafeProtocol[lowerProto]) {
        for (var i = 0, l = autoEscape.length; i < l; i++) {
          var ae = autoEscape[i];
          if (rest.indexOf(ae) === -1) {
            continue;
          }
          var esc = encodeURIComponent(ae);
          if (esc === ae) {
            esc = escape(ae);
          }
          rest = rest.split(ae).join(esc);
        }
      }
      var hash = rest.indexOf("#");
      if (hash !== -1) {
        this.hash = rest.substr(hash);
        rest = rest.slice(0, hash);
      }
      var qm = rest.indexOf("?");
      if (qm !== -1) {
        this.search = rest.substr(qm);
        this.query = rest.substr(qm + 1);
        if (parseQueryString) {
          this.query = querystring.parse(this.query);
        }
        rest = rest.slice(0, qm);
      } else if (parseQueryString) {
        this.search = "";
        this.query = {};
      }
      if (rest) {
        this.pathname = rest;
      }
      if (slashedProtocol[lowerProto] && this.hostname && !this.pathname) {
        this.pathname = "/";
      }
      if (this.pathname || this.search) {
        var p = this.pathname || "";
        var s = this.search || "";
        this.path = p + s;
      }
      this.href = this.format();
      return this;
    };
    function urlFormat(obj) {
      if (typeof obj === "string") {
        obj = urlParse(obj);
      }
      if (!(obj instanceof Url)) {
        return Url.prototype.format.call(obj);
      }
      return obj.format();
    }
    Url.prototype.format = function() {
      var auth = this.auth || "";
      if (auth) {
        auth = encodeURIComponent(auth);
        auth = auth.replace(/%3A/i, ":");
        auth += "@";
      }
      var protocol = this.protocol || "", pathname = this.pathname || "", hash = this.hash || "", host = false, query = "";
      if (this.host) {
        host = auth + this.host;
      } else if (this.hostname) {
        host = auth + (this.hostname.indexOf(":") === -1 ? this.hostname : "[" + this.hostname + "]");
        if (this.port) {
          host += ":" + this.port;
        }
      }
      if (this.query && typeof this.query === "object" && Object.keys(this.query).length) {
        query = querystring.stringify(this.query, {
          arrayFormat: "repeat",
          addQueryPrefix: false
        });
      }
      var search = this.search || query && "?" + query || "";
      if (protocol && protocol.substr(-1) !== ":") {
        protocol += ":";
      }
      if (this.slashes || (!protocol || slashedProtocol[protocol]) && host !== false) {
        host = "//" + (host || "");
        if (pathname && pathname.charAt(0) !== "/") {
          pathname = "/" + pathname;
        }
      } else if (!host) {
        host = "";
      }
      if (hash && hash.charAt(0) !== "#") {
        hash = "#" + hash;
      }
      if (search && search.charAt(0) !== "?") {
        search = "?" + search;
      }
      pathname = pathname.replace(/[?#]/g, function(match) {
        return encodeURIComponent(match);
      });
      search = search.replace("#", "%23");
      return protocol + host + pathname + search + hash;
    };
    function urlResolve(source, relative) {
      return urlParse(source, false, true).resolve(relative);
    }
    Url.prototype.resolve = function(relative) {
      return this.resolveObject(urlParse(relative, false, true)).format();
    };
    function urlResolveObject(source, relative) {
      if (!source) {
        return relative;
      }
      return urlParse(source, false, true).resolveObject(relative);
    }
    Url.prototype.resolveObject = function(relative) {
      if (typeof relative === "string") {
        var rel = new Url();
        rel.parse(relative, false, true);
        relative = rel;
      }
      var result = new Url();
      var tkeys = Object.keys(this);
      for (var tk = 0; tk < tkeys.length; tk++) {
        var tkey = tkeys[tk];
        result[tkey] = this[tkey];
      }
      result.hash = relative.hash;
      if (relative.href === "") {
        result.href = result.format();
        return result;
      }
      if (relative.slashes && !relative.protocol) {
        var rkeys = Object.keys(relative);
        for (var rk = 0; rk < rkeys.length; rk++) {
          var rkey = rkeys[rk];
          if (rkey !== "protocol") {
            result[rkey] = relative[rkey];
          }
        }
        if (slashedProtocol[result.protocol] && result.hostname && !result.pathname) {
          result.pathname = "/";
          result.path = result.pathname;
        }
        result.href = result.format();
        return result;
      }
      if (relative.protocol && relative.protocol !== result.protocol) {
        if (!slashedProtocol[relative.protocol]) {
          var keys = Object.keys(relative);
          for (var v = 0; v < keys.length; v++) {
            var k = keys[v];
            result[k] = relative[k];
          }
          result.href = result.format();
          return result;
        }
        result.protocol = relative.protocol;
        if (!relative.host && !hostlessProtocol[relative.protocol]) {
          var relPath = (relative.pathname || "").split("/");
          while (relPath.length && !(relative.host = relPath.shift())) {
          }
          if (!relative.host) {
            relative.host = "";
          }
          if (!relative.hostname) {
            relative.hostname = "";
          }
          if (relPath[0] !== "") {
            relPath.unshift("");
          }
          if (relPath.length < 2) {
            relPath.unshift("");
          }
          result.pathname = relPath.join("/");
        } else {
          result.pathname = relative.pathname;
        }
        result.search = relative.search;
        result.query = relative.query;
        result.host = relative.host || "";
        result.auth = relative.auth;
        result.hostname = relative.hostname || relative.host;
        result.port = relative.port;
        if (result.pathname || result.search) {
          var p = result.pathname || "";
          var s = result.search || "";
          result.path = p + s;
        }
        result.slashes = result.slashes || relative.slashes;
        result.href = result.format();
        return result;
      }
      var isSourceAbs = result.pathname && result.pathname.charAt(0) === "/", isRelAbs = relative.host || relative.pathname && relative.pathname.charAt(0) === "/", mustEndAbs = isRelAbs || isSourceAbs || result.host && relative.pathname, removeAllDots = mustEndAbs, srcPath = result.pathname && result.pathname.split("/") || [], relPath = relative.pathname && relative.pathname.split("/") || [], psychotic = result.protocol && !slashedProtocol[result.protocol];
      if (psychotic) {
        result.hostname = "";
        result.port = null;
        if (result.host) {
          if (srcPath[0] === "") {
            srcPath[0] = result.host;
          } else {
            srcPath.unshift(result.host);
          }
        }
        result.host = "";
        if (relative.protocol) {
          relative.hostname = null;
          relative.port = null;
          if (relative.host) {
            if (relPath[0] === "") {
              relPath[0] = relative.host;
            } else {
              relPath.unshift(relative.host);
            }
          }
          relative.host = null;
        }
        mustEndAbs = mustEndAbs && (relPath[0] === "" || srcPath[0] === "");
      }
      if (isRelAbs) {
        result.host = relative.host || relative.host === "" ? relative.host : result.host;
        result.hostname = relative.hostname || relative.hostname === "" ? relative.hostname : result.hostname;
        result.search = relative.search;
        result.query = relative.query;
        srcPath = relPath;
      } else if (relPath.length) {
        if (!srcPath) {
          srcPath = [];
        }
        srcPath.pop();
        srcPath = srcPath.concat(relPath);
        result.search = relative.search;
        result.query = relative.query;
      } else if (relative.search != null) {
        if (psychotic) {
          result.host = srcPath.shift();
          result.hostname = result.host;
          var authInHost = result.host && result.host.indexOf("@") > 0 ? result.host.split("@") : false;
          if (authInHost) {
            result.auth = authInHost.shift();
            result.hostname = authInHost.shift();
            result.host = result.hostname;
          }
        }
        result.search = relative.search;
        result.query = relative.query;
        if (result.pathname !== null || result.search !== null) {
          result.path = (result.pathname ? result.pathname : "") + (result.search ? result.search : "");
        }
        result.href = result.format();
        return result;
      }
      if (!srcPath.length) {
        result.pathname = null;
        if (result.search) {
          result.path = "/" + result.search;
        } else {
          result.path = null;
        }
        result.href = result.format();
        return result;
      }
      var last = srcPath.slice(-1)[0];
      var hasTrailingSlash = (result.host || relative.host || srcPath.length > 1) && (last === "." || last === "..") || last === "";
      var up = 0;
      for (var i = srcPath.length; i >= 0; i--) {
        last = srcPath[i];
        if (last === ".") {
          srcPath.splice(i, 1);
        } else if (last === "..") {
          srcPath.splice(i, 1);
          up++;
        } else if (up) {
          srcPath.splice(i, 1);
          up--;
        }
      }
      if (!mustEndAbs && !removeAllDots) {
        for (; up--; up) {
          srcPath.unshift("..");
        }
      }
      if (mustEndAbs && srcPath[0] !== "" && (!srcPath[0] || srcPath[0].charAt(0) !== "/")) {
        srcPath.unshift("");
      }
      if (hasTrailingSlash && srcPath.join("/").substr(-1) !== "/") {
        srcPath.push("");
      }
      var isAbsolute = srcPath[0] === "" || srcPath[0] && srcPath[0].charAt(0) === "/";
      if (psychotic) {
        result.hostname = isAbsolute ? "" : srcPath.length ? srcPath.shift() : "";
        result.host = result.hostname;
        var authInHost = result.host && result.host.indexOf("@") > 0 ? result.host.split("@") : false;
        if (authInHost) {
          result.auth = authInHost.shift();
          result.hostname = authInHost.shift();
          result.host = result.hostname;
        }
      }
      mustEndAbs = mustEndAbs || result.host && srcPath.length;
      if (mustEndAbs && !isAbsolute) {
        srcPath.unshift("");
      }
      if (srcPath.length > 0) {
        result.pathname = srcPath.join("/");
      } else {
        result.pathname = null;
        result.path = null;
      }
      if (result.pathname !== null || result.search !== null) {
        result.path = (result.pathname ? result.pathname : "") + (result.search ? result.search : "");
      }
      result.auth = relative.auth || result.auth;
      result.slashes = result.slashes || relative.slashes;
      result.href = result.format();
      return result;
    };
    Url.prototype.parseHost = function() {
      var host = this.host;
      var port = portPattern.exec(host);
      if (port) {
        port = port[0];
        if (port !== ":") {
          this.port = port.substr(1);
        }
        host = host.substr(0, host.length - port.length);
      }
      if (host) {
        this.hostname = host;
      }
    };
    exports.parse = urlParse;
    exports.resolve = urlResolve;
    exports.resolveObject = urlResolveObject;
    exports.format = urlFormat;
    exports.Url = Url;
  }
});

// node_modules/@readme/httpsnippet/dist/helpers/code-builder.mjs
var DEFAULT_INDENTATION_CHARACTER = "";
var DEFAULT_LINE_JOIN = "\n";
var CodeBuilder = class {
  /**
  * Helper object to format and aggragate lines of code.
  * Lines are aggregated in a `code` array, and need to be joined to obtain a proper code snippet.
  */
  constructor({ indent, join } = {}) {
    this.postProcessors = [];
    this.code = [];
    this.indentationCharacter = DEFAULT_INDENTATION_CHARACTER;
    this.lineJoin = DEFAULT_LINE_JOIN;
    this.indentLine = (line, indentationLevel = 0) => {
      return `${this.indentationCharacter.repeat(indentationLevel)}${line}`;
    };
    this.unshift = (line, indentationLevel) => {
      const newLine = this.indentLine(line, indentationLevel);
      this.code.unshift(newLine);
    };
    this.push = (line, indentationLevel) => {
      const newLine = this.indentLine(line, indentationLevel);
      this.code.push(newLine);
    };
    this.pushToLast = (line) => {
      if (!this.code) this.push(line);
      const updatedLine = `${this.code[this.code.length - 1]}${line}`;
      this.code[this.code.length - 1] = updatedLine;
    };
    this.blank = () => {
      this.code.push("");
    };
    this.join = () => {
      const unreplacedCode = this.code.join(this.lineJoin);
      return this.postProcessors.reduce((accumulator, replacer) => replacer(accumulator), unreplacedCode);
    };
    this.addPostProcessor = (postProcessor) => {
      this.postProcessors = [...this.postProcessors, postProcessor];
    };
    this.indentationCharacter = indent || DEFAULT_INDENTATION_CHARACTER;
    this.lineJoin = join ?? DEFAULT_LINE_JOIN;
  }
};

// node_modules/@readme/httpsnippet/dist/targets-ZiFkWqLU.mjs
var import_stringify_object = __toESM(require_stringify_object(), 1);
var getHeaderName = (headers2, name) => Object.keys(headers2).find((header) => header.toLowerCase() === name.toLowerCase());
var getHeader = (headers2, name) => {
  const headerName = getHeaderName(headers2, name);
  if (!headerName) return;
  return headers2[headerName];
};
var hasHeader = (headers2, name) => Boolean(getHeaderName(headers2, name));
var isMimeTypeJSON = (mimeType) => [
  "application/json",
  "application/x-json",
  "text/json",
  "text/x-json",
  "+json"
].some((type) => mimeType.indexOf(type) > -1);
var agent = {
  info: {
    key: "agent",
    title: "Agent",
    default: "prompt"
  },
  clientsById: { prompt: {
    info: {
      key: "prompt",
      title: "Agent Prompt",
      link: "https://github.com/readmeio/httpsnippet",
      description: "A copy-and-pastable prompt, for any AI agent, describing how to make the request.",
      extname: ".txt"
    },
    convert: ({ method, fullUrl, allHeaders, postData }, options) => {
      const opts = {
        indent: "  ",
        join: "\n",
        ...options
      };
      const { blank, push, join } = new CodeBuilder(opts);
      push("Write code that makes the HTTP request described below. Use my preferred programming language and HTTP client library \u2014 if I haven't told you what those are, ask me before writing any code.");
      blank();
      push(`Method: ${method}`);
      push(`URL: ${fullUrl}`);
      const headerKeys = Object.keys(allHeaders);
      if (headerKeys.length) {
        blank();
        push("Headers:");
        headerKeys.forEach((key) => {
          push(`${key}: ${allHeaders[key]}`, 1);
        });
      }
      if (postData.text) {
        blank();
        push(`Body (${postData.mimeType}):`);
        push(postData.text);
      }
      blank();
      push("The request the code makes must match the method, URL, headers, and body exactly as described above.");
      if (opts.markdownURL) {
        blank();
        push(`Check ${opts.markdownURL} for more info.`);
      }
      return join();
    }
  } }
};
function escapeString(rawValue, options = {}) {
  const { delimiter = '"', escapeChar = "\\", escapeNewlines = true } = options;
  return [...rawValue.toString()].map((c2) => {
    if (c2 === "\b") return `${escapeChar}b`;
    else if (c2 === "	") return `${escapeChar}t`;
    else if (c2 === "\n") {
      if (escapeNewlines) return `${escapeChar}n`;
      return c2;
    } else if (c2 === "\f") return `${escapeChar}f`;
    else if (c2 === "\r") {
      if (escapeNewlines) return `${escapeChar}r`;
      return c2;
    } else if (c2 === escapeChar) return escapeChar + escapeChar;
    else if (c2 === delimiter) return escapeChar + delimiter;
    else if (c2 < " " || c2 > "~") return JSON.stringify(c2).slice(1, -1);
    return c2;
  }).join("");
}
var escapeForSingleQuotes = (value) => escapeString(value, { delimiter: "'" });
var escapeForDoubleQuotes = (value) => escapeString(value, { delimiter: '"' });
var csharpString = (value) => `"${escapeForDoubleQuotes(value).replace(/[\u0085\u2028\u2029]/g, (c2) => `\\u${c2.charCodeAt(0).toString(16).padStart(4, "0")}`)}"`;
var c = {
  info: {
    key: "c",
    title: "C",
    default: "libcurl",
    cli: "c"
  },
  clientsById: { libcurl: {
    info: {
      key: "libcurl",
      title: "Libcurl",
      link: "http://curl.haxx.se/libcurl",
      description: "Simple REST and HTTP API Client for C",
      extname: ".c"
    },
    convert: ({ method, fullUrl, headersObj, allHeaders, postData }) => {
      const { push, blank, join } = new CodeBuilder();
      push("CURL *hnd = curl_easy_init();");
      blank();
      push(`curl_easy_setopt(hnd, CURLOPT_CUSTOMREQUEST, "${method.toUpperCase()}");`);
      push("curl_easy_setopt(hnd, CURLOPT_WRITEDATA, stdout);");
      push(`curl_easy_setopt(hnd, CURLOPT_URL, "${fullUrl}");`);
      const headers2 = Object.keys(headersObj);
      if (headers2.length) {
        blank();
        push("struct curl_slist *headers = NULL;");
        headers2.forEach((header) => {
          push(`headers = curl_slist_append(headers, "${header}: ${escapeForDoubleQuotes(headersObj[header])}");`);
        });
        push("curl_easy_setopt(hnd, CURLOPT_HTTPHEADER, headers);");
      }
      if (allHeaders.cookie) {
        blank();
        push(`curl_easy_setopt(hnd, CURLOPT_COOKIE, "${allHeaders.cookie}");`);
      }
      if (postData.text) {
        blank();
        push(`curl_easy_setopt(hnd, CURLOPT_POSTFIELDS, ${JSON.stringify(postData.text)});`);
      }
      blank();
      push("CURLcode ret = curl_easy_perform(hnd);");
      return join();
    }
  } }
};
var Keyword = class {
  constructor(name) {
    this.name = "";
    this.toString = () => `:${this.name}`;
    this.name = name;
  }
};
var File = class {
  constructor(path) {
    this.path = "";
    this.toString = () => `(clojure.java.io/file "${this.path}")`;
    this.path = path;
  }
};
var jsType = (input) => {
  if (input === void 0) return null;
  if (input === null) return "null";
  return input.constructor.name.toLowerCase();
};
var objEmpty = (input) => {
  if (input === void 0) return true;
  else if (jsType(input) === "object") return Object.keys(input).length === 0;
  return false;
};
var filterEmpty = (input) => {
  Object.keys(input).filter((x) => objEmpty(input[x])).forEach((x) => {
    delete input[x];
  });
  return input;
};
var padBlock = (padSize, input) => {
  const padding = " ".repeat(padSize);
  return input.replace(/\n/g, `
${padding}`);
};
var jsToEdn = (js) => {
  switch (jsType(js)) {
    case "string":
      return `"${js.replace(/"/g, '\\"')}"`;
    case "file":
      return js.toString();
    case "keyword":
      return js.toString();
    case "null":
      return "nil";
    case "regexp":
      return `#"${js.source}"`;
    case "object": {
      const obj = Object.keys(js).reduce((accumulator, key) => {
        return `${accumulator}:${key} ${padBlock(key.length + 2, jsToEdn(js[key]))}
 `;
      }, "").trim();
      return `{${padBlock(1, obj)}}`;
    }
    case "array": {
      const arr = js.reduce((accumulator, value) => `${accumulator} ${jsToEdn(value)}`, "").trim();
      return `[${padBlock(1, arr)}]`;
    }
    default:
      return js.toString();
  }
};
var clojure = {
  info: {
    key: "clojure",
    title: "Clojure",
    default: "clj_http"
  },
  clientsById: { clj_http: {
    info: {
      key: "clj_http",
      title: "clj-http",
      link: "https://github.com/dakrone/clj-http",
      description: "An idiomatic clojure http client wrapping the apache client.",
      extname: ".clj"
    },
    convert: ({ queryObj, method, postData, url, allHeaders }, options) => {
      const { push, join } = new CodeBuilder({ indent: options?.indent });
      const methods = [
        "get",
        "post",
        "put",
        "delete",
        "patch",
        "head",
        "options"
      ];
      method = method.toLowerCase();
      if (!methods.includes(method)) {
        push("Method not supported");
        return join();
      }
      const params2 = {
        headers: allHeaders,
        "query-params": queryObj
      };
      switch (postData.mimeType) {
        case "application/json":
          {
            params2["content-type"] = new Keyword("json");
            params2["form-params"] = postData.jsonObj;
            const header = getHeaderName(params2.headers, "content-type");
            if (header) delete params2.headers[header];
          }
          break;
        case "application/x-www-form-urlencoded":
          {
            params2["form-params"] = postData.paramsObj;
            const header = getHeaderName(params2.headers, "content-type");
            if (header) delete params2.headers[header];
          }
          break;
        case "text/plain":
          {
            params2.body = postData.text;
            const header = getHeaderName(params2.headers, "content-type");
            if (header) delete params2.headers[header];
          }
          break;
        case "multipart/form-data":
          if (postData.params) {
            params2.multipart = postData.params.map((param) => {
              if (param.fileName && !param.value) return {
                name: param.name,
                content: new File(param.fileName)
              };
              return {
                name: param.name,
                content: param.value
              };
            });
            const header = getHeaderName(params2.headers, "content-type");
            if (header) delete params2.headers[header];
          }
          break;
      }
      switch (getHeader(params2.headers, "accept")) {
        case "application/json":
          {
            params2.accept = new Keyword("json");
            const header = getHeaderName(params2.headers, "accept");
            if (header) delete params2.headers[header];
          }
          break;
      }
      push("(require '[clj-http.client :as client])\n");
      if (objEmpty(filterEmpty(params2))) push(`(client/${method} "${url}")`);
      else {
        const padding = 11 + method.length + url.length;
        const formattedParams = padBlock(padding, jsToEdn(filterEmpty(params2)));
        push(`(client/${method} "${url}" ${formattedParams})`);
      }
      return join();
    }
  } }
};
var crystal = {
  info: {
    key: "crystal",
    title: "Crystal",
    default: "native"
  },
  clientsById: { native: {
    info: {
      key: "native",
      title: "http::client",
      link: "https://crystal-lang.org/api/master/HTTP/Client.html",
      description: "Crystal HTTP client",
      extname: ".cr"
    },
    convert: ({ method: rawMethod, fullUrl, postData, allHeaders }, options = {}) => {
      const { insecureSkipVerify = false } = options;
      const { push, blank, join } = new CodeBuilder();
      push('require "http/client"');
      blank();
      push(`url = "${fullUrl}"`);
      const headers2 = Object.keys(allHeaders);
      if (headers2.length) {
        push("headers = HTTP::Headers{");
        headers2.forEach((key) => {
          push(`  "${key}" => "${escapeForDoubleQuotes(allHeaders[key])}"`);
        });
        push("}");
      }
      if (postData.text) push(`reqBody = ${JSON.stringify(postData.text)}`);
      blank();
      const method = rawMethod.toUpperCase();
      const methods = [
        "GET",
        "POST",
        "HEAD",
        "DELETE",
        "PATCH",
        "PUT",
        "OPTIONS"
      ];
      const headersContext = headers2.length ? ", headers: headers" : "";
      const bodyContext = postData.text ? ", body: reqBody" : "";
      const sslContext = insecureSkipVerify ? ", tls: OpenSSL::SSL::Context::Client.insecure" : "";
      if (methods.includes(method)) push(`response = HTTP::Client.${method.toLowerCase()} url${headersContext}${bodyContext}${sslContext}`);
      else push(`response = HTTP::Client.exec "${method}", url${headersContext}${bodyContext}${sslContext}`);
      push("puts response.body");
      return join();
    }
  } }
};
var getDecompressionMethods = (allHeaders) => {
  let acceptEncodings = getHeader(allHeaders, "accept-encoding");
  if (!acceptEncodings) return [];
  const supportedMethods2 = {
    gzip: "DecompressionMethods.GZip",
    deflate: "DecompressionMethods.Deflate"
  };
  const methods = [];
  if (typeof acceptEncodings === "string") acceptEncodings = [acceptEncodings];
  acceptEncodings.forEach((acceptEncoding) => {
    acceptEncoding.split(",").forEach((encoding) => {
      const match = /\s*([^;\s]+)/.exec(encoding);
      if (match) {
        const method = supportedMethods2[match[1]];
        if (method) methods.push(method);
      }
    });
  });
  return methods;
};
var httpclient = {
  info: {
    key: "httpclient",
    title: "HttpClient",
    link: "https://docs.microsoft.com/en-us/dotnet/api/system.net.http.httpclient",
    description: ".NET Standard HTTP Client",
    extname: ".cs"
  },
  convert: ({ allHeaders, postData, method, fullUrl }, options) => {
    const { push, join } = new CodeBuilder({ indent: {
      indent: "    ",
      ...options
    }.indent });
    push("using System.Net.Http.Headers;");
    let clienthandler = "";
    const cookies = Boolean(allHeaders.cookie);
    const decompressionMethods = getDecompressionMethods(allHeaders);
    if (cookies || decompressionMethods.length) {
      clienthandler = "clientHandler";
      push("var clientHandler = new HttpClientHandler");
      push("{");
      if (cookies) push("UseCookies = false,", 1);
      if (decompressionMethods.length) push(`AutomaticDecompression = ${decompressionMethods.join(" | ")},`, 1);
      push("};");
    }
    push(`var client = new HttpClient(${clienthandler});`);
    push("var request = new HttpRequestMessage");
    push("{");
    const methods = [
      "GET",
      "POST",
      "PUT",
      "DELETE",
      "PATCH",
      "HEAD",
      "OPTIONS",
      "TRACE"
    ];
    method = method.toUpperCase();
    if (method && methods.includes(method)) method = `HttpMethod.${method[0]}${method.substring(1).toLowerCase()}`;
    else method = `new HttpMethod(${csharpString(method)})`;
    push(`Method = ${method},`, 1);
    push(`RequestUri = new Uri(${csharpString(fullUrl)}),`, 1);
    const headers2 = Object.keys(allHeaders).filter((header) => {
      switch (header.toLowerCase()) {
        case "content-type":
        case "content-length":
        case "accept-encoding":
          return false;
        default:
          return true;
      }
    });
    if (headers2.length) {
      push("Headers =", 1);
      push("{", 1);
      headers2.forEach((key) => {
        push(`{ ${csharpString(key)}, ${csharpString(allHeaders[key])} },`, 2);
      });
      push("},", 1);
    }
    if (postData.text) {
      const contentType = postData.mimeType;
      switch (contentType) {
        case "application/x-www-form-urlencoded":
          push("Content = new FormUrlEncodedContent(new List<KeyValuePair<string, string>>", 1);
          push("{", 1);
          postData.params?.forEach((param) => {
            push(`new(${csharpString(param.name)}, ${csharpString(param.value || "")}),`, 2);
          });
          push("}),", 1);
          break;
        case "multipart/form-data":
          push("Content = new MultipartFormDataContent", 1);
          push("{", 1);
          postData.params?.forEach((param) => {
            push(`new StringContent(${csharpString(param.value || "")})`, 2);
            push("{", 2);
            push("Headers =", 3);
            push("{", 3);
            if (param.contentType) push(`ContentType = MediaTypeHeaderValue.Parse(${csharpString(param.contentType)}),`, 4);
            push('ContentDisposition = new ContentDispositionHeaderValue("form-data")', 4);
            push("{", 4);
            push(`Name = ${csharpString(param.name)},`, 5);
            if (param.fileName) push(`FileName = ${csharpString(param.fileName)},`, 5);
            push("}", 4);
            push("}", 3);
            push("},", 2);
          });
          push("},", 1);
          break;
        default:
          push(`Content = new StringContent(${csharpString(postData.text || "")})`, 1);
          push("{", 1);
          push("Headers =", 2);
          push("{", 2);
          push(`ContentType = MediaTypeHeaderValue.Parse(${csharpString(contentType)})`, 3);
          push("}", 2);
          push("}", 1);
          break;
      }
    }
    push("};");
    push("using (var response = await client.SendAsync(request))");
    push("{");
    push("response.EnsureSuccessStatusCode();", 1);
    push("var body = await response.Content.ReadAsStringAsync();", 1);
    push("Console.WriteLine(body);", 1);
    push("}");
    return join();
  }
};
function title(s) {
  return s[0].toUpperCase() + s.slice(1).toLowerCase();
}
var csharp = {
  info: {
    key: "csharp",
    title: "C#",
    default: "restsharp",
    cli: "dotnet"
  },
  clientsById: {
    httpclient,
    restsharp: {
      info: {
        key: "restsharp",
        title: "RestSharp",
        link: "http://restsharp.org/",
        description: "Simple REST and HTTP API Client for .NET",
        extname: ".cs",
        installation: () => "dotnet add package RestSharp"
      },
      convert: ({ method, fullUrl, headersObj, cookies, postData, uriObj }) => {
        const { push, join } = new CodeBuilder();
        if (![
          "GET",
          "POST",
          "PUT",
          "DELETE",
          "PATCH",
          "HEAD",
          "OPTIONS"
        ].includes(method.toUpperCase())) return "Method not supported";
        push("using RestSharp;\n\n");
        push(`var options = new RestClientOptions("${fullUrl}");`);
        push("var client = new RestClient(options);");
        push('var request = new RestRequest("");');
        const isMultipart2 = postData.mimeType && postData.mimeType === "multipart/form-data";
        if (isMultipart2) push("request.AlwaysMultipartFormData = true;");
        Object.keys(headersObj).forEach((key) => {
          if (postData.mimeType && key.toLowerCase() === "content-type" && postData.text) {
            if (isMultipart2 && postData.boundary) push(`request.FormBoundary = "${postData.boundary}";`);
            return;
          }
          push(`request.AddHeader("${key}", "${escapeForDoubleQuotes(headersObj[key])}");`);
        });
        cookies.forEach(({ name, value }) => {
          push(`request.AddCookie("${name}", "${escapeForDoubleQuotes(value)}", "${uriObj.pathname}", "${uriObj.host}");`);
        });
        switch (postData.mimeType) {
          case "multipart/form-data":
            if (!postData.params) break;
            postData.params.forEach((param) => {
              if (param.fileName) push(`request.AddFile("${param.name}", "${param.fileName}");`);
              else push(`request.AddParameter("${param.name}", "${param.value}");`);
            });
            break;
          case "application/x-www-form-urlencoded":
            if (!postData.params) break;
            postData.params.forEach((param) => {
              push(`request.AddParameter("${param.name}", "${param.value}");`);
            });
            break;
          case "application/json":
            if (!postData.text) break;
            push(`request.AddJsonBody(${JSON.stringify(postData.text)}, false);`);
            break;
          default:
            if (!postData.text) break;
            push(`request.AddStringBody("${postData.text}", "${postData.mimeType}");`);
        }
        push(`var response = await client.${title(method)}Async(request);
`);
        push('Console.WriteLine("{0}", response.Content);\n');
        return join();
      }
    }
  }
};
var go = {
  info: {
    key: "go",
    title: "Go",
    default: "native",
    cli: "go"
  },
  clientsById: { native: {
    info: {
      key: "native",
      title: "NewRequest",
      link: "http://golang.org/pkg/net/http/#NewRequest",
      description: "Golang HTTP client request",
      extname: ".go"
    },
    convert: ({ postData, method, allHeaders, fullUrl }, options = {}) => {
      const { blank, push, join } = new CodeBuilder({ indent: "	" });
      const { showBoilerplate = true, checkErrors = false, printBody = true, timeout = -1, insecureSkipVerify = false } = options;
      const errorPlaceholder = checkErrors ? "err" : "_";
      const indent = showBoilerplate ? 1 : 0;
      const errorCheck = () => {
        if (checkErrors) {
          push("if err != nil {", indent);
          push("panic(err)", indent + 1);
          push("}", indent);
        }
      };
      if (showBoilerplate) {
        push("package main");
        blank();
        push("import (");
        push('"fmt"', indent);
        if (timeout > 0) push('"time"', indent);
        if (insecureSkipVerify) push('"crypto/tls"', indent);
        if (postData.text) push('"strings"', indent);
        push('"net/http"', indent);
        if (printBody) push('"io"', indent);
        push(")");
        blank();
        push("func main() {");
        blank();
      }
      if (insecureSkipVerify) {
        push("insecureTransport := http.DefaultTransport.(*http.Transport).Clone()", indent);
        push("insecureTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}", indent);
      }
      const hasTimeout = timeout > 0;
      const hasClient = hasTimeout || insecureSkipVerify;
      const client = hasClient ? "client" : "http.DefaultClient";
      if (hasClient) {
        push("client := http.Client{", indent);
        if (hasTimeout) push(`Timeout: time.Duration(${timeout} * time.Second),`, indent + 1);
        if (insecureSkipVerify) push("Transport: insecureTransport,", indent + 1);
        push("}", indent);
        blank();
      }
      push(`url := "${escapeForDoubleQuotes(fullUrl)}"`, indent);
      blank();
      if (postData.text) {
        push(`payload := strings.NewReader(${JSON.stringify(postData.text)})`, indent);
        blank();
        push(`req, ${errorPlaceholder} := http.NewRequest("${escapeForDoubleQuotes(method)}", url, payload)`, indent);
        blank();
      } else {
        push(`req, ${errorPlaceholder} := http.NewRequest("${escapeForDoubleQuotes(method)}", url, nil)`, indent);
        blank();
      }
      errorCheck();
      if (Object.keys(allHeaders).length) {
        Object.keys(allHeaders).forEach((key) => {
          push(`req.Header.Add("${escapeForDoubleQuotes(key)}", "${escapeForDoubleQuotes(allHeaders[key])}")`, indent);
        });
        blank();
      }
      push(`res, ${errorPlaceholder} := ${client}.Do(req)`, indent);
      errorCheck();
      if (printBody) {
        blank();
        push("defer res.Body.Close()", indent);
        push(`body, ${errorPlaceholder} := io.ReadAll(res.Body)`, indent);
        errorCheck();
      }
      blank();
      if (printBody) push("fmt.Println(string(body))", indent);
      if (showBoilerplate) {
        blank();
        push("}");
      }
      return join();
    }
  } }
};
var CRLF = "\r\n";
var http = {
  info: {
    key: "http",
    title: "HTTP",
    default: "http1.1"
  },
  clientsById: { "http1.1": {
    info: {
      key: "http1.1",
      title: "HTTP/1.1",
      link: "https://tools.ietf.org/html/rfc7230",
      description: "HTTP/1.1 request string in accordance with RFC 7230",
      extname: null
    },
    convert: ({ method, fullUrl, uriObj, httpVersion, allHeaders, postData }, options) => {
      const opts = {
        absoluteURI: false,
        autoContentLength: true,
        autoHost: true,
        ...options
      };
      const { blank, push, join } = new CodeBuilder({
        indent: "",
        join: CRLF
      });
      push(`${method} ${opts.absoluteURI ? fullUrl : uriObj.path} ${httpVersion}`);
      const headerKeys = Object.keys(allHeaders);
      headerKeys.forEach((key) => {
        const keyCapitalized = key.toLowerCase().replace(/(^|-)(\w)/g, (input) => input.toUpperCase());
        push(`${keyCapitalized}: ${allHeaders[key]}`);
      });
      if (opts.autoHost && !headerKeys.includes("host")) push(`Host: ${uriObj.host}`);
      if (opts.autoContentLength && postData.text && !headerKeys.includes("content-length")) push(`Content-Length: ${Buffer.byteLength(postData.text, "ascii").toString()}`);
      blank();
      const headerSection = join();
      const messageBody = postData.text || "";
      return `${headerSection}${CRLF}${messageBody}`;
    }
  } }
};
var java = {
  info: {
    key: "java",
    title: "Java",
    default: "unirest"
  },
  clientsById: {
    asynchttp: {
      info: {
        key: "asynchttp",
        title: "AsyncHttp",
        link: "https://github.com/AsyncHttpClient/async-http-client",
        description: "Asynchronous Http and WebSocket Client library for Java",
        extname: ".java"
      },
      convert: ({ method, allHeaders, postData, fullUrl }, options) => {
        const { blank, push, join } = new CodeBuilder({ indent: {
          indent: "  ",
          ...options
        }.indent });
        push("AsyncHttpClient client = new DefaultAsyncHttpClient();");
        push(`client.prepare("${method.toUpperCase()}", "${fullUrl}")`);
        Object.keys(allHeaders).forEach((key) => {
          push(`.setHeader("${key}", "${escapeForDoubleQuotes(allHeaders[key])}")`, 1);
        });
        if (postData.text) push(`.setBody(${JSON.stringify(postData.text)})`, 1);
        push(".execute()", 1);
        push(".toCompletableFuture()", 1);
        push(".thenAccept(System.out::println)", 1);
        push(".join();", 1);
        blank();
        push("client.close();");
        return join();
      }
    },
    nethttp: {
      info: {
        key: "nethttp",
        title: "java.net.http",
        link: "https://openjdk.java.net/groups/net/httpclient/intro.html",
        description: "Java Standardized HTTP Client API",
        extname: ".java"
      },
      convert: ({ allHeaders, fullUrl, method, postData }, options) => {
        const { push, join } = new CodeBuilder({ indent: {
          indent: "  ",
          ...options
        }.indent });
        push("HttpRequest request = HttpRequest.newBuilder()");
        push(`.uri(URI.create("${escapeForDoubleQuotes(fullUrl)}"))`, 2);
        Object.keys(allHeaders).forEach((key) => {
          push(`.header("${escapeForDoubleQuotes(key)}", "${escapeForDoubleQuotes(allHeaders[key])}")`, 2);
        });
        if (postData.text) push(`.method("${escapeForDoubleQuotes(method.toUpperCase())}", HttpRequest.BodyPublishers.ofString(${JSON.stringify(postData.text)}))`, 2);
        else push(`.method("${escapeForDoubleQuotes(method.toUpperCase())}", HttpRequest.BodyPublishers.noBody())`, 2);
        push(".build();", 2);
        push("HttpResponse<String> response = HttpClient.newHttpClient().send(request, HttpResponse.BodyHandlers.ofString());");
        push("System.out.println(response.body());");
        return join();
      }
    },
    okhttp: {
      info: {
        key: "okhttp",
        title: "OkHttp",
        link: "http://square.github.io/okhttp/",
        description: "An HTTP Request Client Library",
        extname: ".java"
      },
      convert: ({ postData, method, fullUrl, allHeaders }, options) => {
        const { push, blank, join } = new CodeBuilder({ indent: {
          indent: "  ",
          ...options
        }.indent });
        const methods = [
          "GET",
          "POST",
          "PUT",
          "DELETE",
          "PATCH",
          "HEAD"
        ];
        const methodsWithBody = [
          "POST",
          "PUT",
          "DELETE",
          "PATCH"
        ];
        push("OkHttpClient client = new OkHttpClient();");
        blank();
        if (postData.text) {
          if (postData.boundary) push(`MediaType mediaType = MediaType.parse("${escapeForDoubleQuotes(`${postData.mimeType}; boundary=${postData.boundary}`)}");`);
          else push(`MediaType mediaType = MediaType.parse("${escapeForDoubleQuotes(postData.mimeType)}");`);
          push(`RequestBody body = RequestBody.create(mediaType, ${JSON.stringify(postData.text)});`);
        }
        push("Request request = new Request.Builder()");
        push(`.url("${escapeForDoubleQuotes(fullUrl)}")`, 1);
        if (!methods.includes(method.toUpperCase())) if (postData.text) push(`.method("${escapeForDoubleQuotes(method.toUpperCase())}", body)`, 1);
        else push(`.method("${escapeForDoubleQuotes(method.toUpperCase())}", null)`, 1);
        else if (methodsWithBody.includes(method.toUpperCase())) if (postData.text) push(`.${method.toLowerCase()}(body)`, 1);
        else push(`.${method.toLowerCase()}(null)`, 1);
        else push(`.${method.toLowerCase()}()`, 1);
        Object.keys(allHeaders).forEach((key) => {
          push(`.addHeader("${escapeForDoubleQuotes(key)}", "${escapeForDoubleQuotes(allHeaders[key])}")`, 1);
        });
        push(".build();", 1);
        blank();
        push("Response response = client.newCall(request).execute();");
        return join();
      }
    },
    unirest: {
      info: {
        key: "unirest",
        title: "Unirest",
        link: "http://unirest.io/java.html",
        description: "Lightweight HTTP Request Client Library",
        extname: ".java"
      },
      convert: ({ method, allHeaders, postData, fullUrl }, options) => {
        const { join, push } = new CodeBuilder({ indent: {
          indent: "  ",
          ...options
        }.indent });
        if (![
          "GET",
          "POST",
          "PUT",
          "DELETE",
          "PATCH",
          "HEAD",
          "OPTIONS"
        ].includes(method.toUpperCase())) push(`HttpResponse<String> response = Unirest.customMethod("${method.toUpperCase()}","${fullUrl}")`);
        else push(`HttpResponse<String> response = Unirest.${method.toLowerCase()}("${fullUrl}")`);
        Object.keys(allHeaders).forEach((key) => {
          push(`.header("${key}", "${escapeForDoubleQuotes(allHeaders[key])}")`, 1);
        });
        if (postData.text) push(`.body(${JSON.stringify(postData.text)})`, 1);
        push(".asString();", 1);
        return join();
      }
    }
  }
};
var javascript = {
  info: {
    key: "javascript",
    title: "JavaScript",
    default: "fetch"
  },
  clientsById: {
    xhr: {
      info: {
        key: "xhr",
        title: "XMLHttpRequest",
        link: "https://developer.mozilla.org/en-US/docs/Web/API/XMLHttpRequest",
        description: "W3C Standard API that provides scripted client functionality",
        extname: ".js"
      },
      convert: ({ postData, allHeaders, method, fullUrl }, options) => {
        const opts = {
          indent: "  ",
          cors: true,
          ...options
        };
        const { blank, push, join } = new CodeBuilder({ indent: opts.indent });
        switch (postData.mimeType) {
          case "application/json":
            push(`const data = JSON.stringify(${(0, import_stringify_object.default)(postData.jsonObj, { indent: opts.indent })});`);
            blank();
            break;
          case "multipart/form-data":
            if (!postData.params) break;
            push("const data = new FormData();");
            postData.params.forEach((param) => {
              push(`data.append('${param.name}', '${param.value || param.fileName || ""}');`);
            });
            if (hasHeader(allHeaders, "content-type")) {
              if (getHeader(allHeaders, "content-type")?.includes("boundary")) {
                const headerName = getHeaderName(allHeaders, "content-type");
                if (headerName) delete allHeaders[headerName];
              }
            }
            blank();
            break;
          default:
            push(`const data = ${postData.text ? `'${postData.text}'` : "null"};`);
            blank();
        }
        push("const xhr = new XMLHttpRequest();");
        if (opts.cors) push("xhr.withCredentials = true;");
        blank();
        push("xhr.addEventListener('readystatechange', function () {");
        push("if (this.readyState === this.DONE) {", 1);
        push("console.log(this.responseText);", 2);
        push("}", 1);
        push("});");
        blank();
        push(`xhr.open('${method}', '${fullUrl}');`);
        Object.keys(allHeaders).forEach((key) => {
          push(`xhr.setRequestHeader('${key}', '${escapeForSingleQuotes(allHeaders[key])}');`);
        });
        blank();
        push("xhr.send(data);");
        return join();
      }
    },
    axios: {
      info: {
        key: "axios",
        title: "Axios",
        link: "https://github.com/axios/axios",
        description: "Promise based HTTP client for the browser and node.js",
        extname: ".js",
        installation: () => "npm install axios --save"
      },
      convert: ({ allHeaders, method, url, queryObj, postData }, options) => {
        const { blank, push, join, addPostProcessor } = new CodeBuilder({ indent: {
          indent: "  ",
          ...options
        }.indent });
        push("import axios from 'axios';");
        blank();
        const requestOptions = {
          method,
          url
        };
        if (Object.keys(queryObj).length) requestOptions.params = queryObj;
        if (Object.keys(allHeaders).length) requestOptions.headers = allHeaders;
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            if (postData.params) {
              push("const encodedParams = new URLSearchParams();");
              postData.params.forEach((param) => {
                push(`encodedParams.set('${param.name}', '${param.value}');`);
              });
              blank();
              requestOptions.data = "encodedParams,";
              addPostProcessor((code) => code.replace(/'encodedParams,'/, "encodedParams,"));
            }
            break;
          case "application/json":
            if (postData.jsonObj) requestOptions.data = postData.jsonObj;
            break;
          case "multipart/form-data":
            if (!postData.params) break;
            push("const form = new FormData();");
            postData.params.forEach((param) => {
              push(`form.append('${param.name}', '${param.value || param.fileName || ""}');`);
            });
            blank();
            requestOptions.data = "[form]";
            break;
          default:
            if (postData.text) requestOptions.data = postData.text;
        }
        push(`const options = ${(0, import_stringify_object.default)(requestOptions, {
          indent: "  ",
          inlineCharacterLimit: 80
        }).replace('"[form]"', "form")};`);
        blank();
        push("try {");
        push("const { data } = await axios.request(options);", 1);
        push("console.log(data);", 1);
        push("} catch (error) {");
        push("console.error(error);", 1);
        push("}");
        return join();
      }
    },
    fetch: {
      info: {
        key: "fetch",
        title: "fetch",
        link: "https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch",
        description: "Perform asynchronous HTTP requests with the Fetch API",
        extname: ".js"
      },
      convert: ({ method, allHeaders, postData, fullUrl }, inputOpts) => {
        const opts = {
          indent: "  ",
          credentials: null,
          ...inputOpts
        };
        const { blank, join, push } = new CodeBuilder({ indent: opts.indent });
        const options = { method };
        if (Object.keys(allHeaders).length) options.headers = allHeaders;
        if (opts.credentials !== null) options.credentials = opts.credentials;
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            options.body = postData.paramsObj ? postData.paramsObj : postData.text;
            break;
          case "application/json":
            if (postData.jsonObj) options.body = postData.jsonObj;
            break;
          case "multipart/form-data": {
            if (!postData.params) break;
            const contentTypeHeader = getHeaderName(allHeaders, "content-type");
            if (contentTypeHeader) delete allHeaders[contentTypeHeader];
            push("const form = new FormData();");
            postData.params.forEach((param) => {
              push(`form.append('${param.name}', '${param.value || param.fileName || ""}');`);
            });
            blank();
            break;
          }
          default:
            if (postData.text) options.body = postData.text;
        }
        if (options.headers && !Object.keys(options.headers).length) delete options.headers;
        push(`const options = ${(0, import_stringify_object.default)(options, {
          indent: opts.indent,
          inlineCharacterLimit: 80,
          transform: (object, property, originalResult) => {
            if (property === "body") {
              if (postData.mimeType === "application/x-www-form-urlencoded") return `new URLSearchParams(${originalResult})`;
              else if (postData.mimeType === "application/json") return `JSON.stringify(${originalResult})`;
            }
            return originalResult;
          }
        })};`);
        blank();
        if (postData.params && postData.mimeType === "multipart/form-data") {
          push("options.body = form;");
          blank();
        }
        push(`fetch('${fullUrl}', options)`);
        push(".then(res => res.json())", 1);
        push(".then(res => console.log(res))", 1);
        push(".catch(err => console.error(err));", 1);
        return join();
      }
    },
    jquery: {
      info: {
        key: "jquery",
        title: "jQuery",
        link: "http://api.jquery.com/jquery.ajax/",
        description: "Perform an asynchronous HTTP (Ajax) requests with jQuery",
        extname: ".js"
      },
      convert: ({ fullUrl, method, allHeaders, postData }, options) => {
        const opts = {
          indent: "  ",
          ...options
        };
        const { blank, push, join } = new CodeBuilder({ indent: opts.indent });
        const settings = {
          async: true,
          crossDomain: true,
          url: fullUrl,
          method,
          headers: allHeaders
        };
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            settings.data = postData.paramsObj ? postData.paramsObj : postData.text;
            break;
          case "application/json":
            settings.processData = false;
            settings.data = postData.text;
            break;
          case "multipart/form-data":
            if (!postData.params) break;
            push("const form = new FormData();");
            postData.params.forEach((param) => {
              push(`form.append('${param.name}', '${param.value || param.fileName || ""}');`);
            });
            settings.processData = false;
            settings.contentType = false;
            settings.mimeType = "multipart/form-data";
            settings.data = "[form]";
            if (hasHeader(allHeaders, "content-type")) {
              if (getHeader(allHeaders, "content-type")?.includes("boundary")) {
                const headerName = getHeaderName(allHeaders, "content-type");
                if (headerName) delete settings.headers[headerName];
              }
            }
            blank();
            break;
          default:
            if (postData.text) settings.data = postData.text;
        }
        push(`const settings = ${(0, import_stringify_object.default)(settings, { indent: opts.indent }).replace("'[form]'", "form")};`);
        blank();
        push("$.ajax(settings).done(res => {");
        push("console.log(res);", 1);
        push("});");
        return join();
      }
    }
  }
};
var json = {
  info: {
    key: "json",
    title: "JSON",
    default: "native"
  },
  clientsById: { native: {
    info: {
      key: "native",
      title: "Native JSON",
      link: "https://www.json.org/json-en.html",
      description: "A JSON represetation of any HAR payload.",
      extname: ".json"
    },
    convert: ({ postData }, inputOpts) => {
      const opts = {
        indent: "  ",
        ...inputOpts
      };
      let payload = "";
      switch (postData.mimeType) {
        case "application/x-www-form-urlencoded":
          payload = postData.paramsObj ? postData.paramsObj : postData.text;
          break;
        case "application/json":
          if (postData.jsonObj) payload = postData.jsonObj;
          break;
        case "multipart/form-data": {
          if (!postData.params) break;
          const multipartPayload = {};
          postData.params.forEach((param) => {
            multipartPayload[param.name] = param.value;
          });
          payload = multipartPayload;
          break;
        }
        default:
          if (postData.text) payload = postData.text;
      }
      if (typeof payload === "undefined" || payload === "") return "No JSON body";
      return JSON.stringify(payload, null, opts.indent);
    }
  } }
};
var kotlin = {
  info: {
    key: "kotlin",
    title: "Kotlin",
    default: "okhttp"
  },
  clientsById: { okhttp: {
    info: {
      key: "okhttp",
      title: "OkHttp",
      link: "http://square.github.io/okhttp/",
      description: "An HTTP Request Client Library",
      extname: ".kt"
    },
    convert: ({ postData, fullUrl, method, allHeaders }, options) => {
      const { blank, join, push } = new CodeBuilder({ indent: {
        indent: "  ",
        ...options
      }.indent });
      const methods = [
        "GET",
        "POST",
        "PUT",
        "DELETE",
        "PATCH",
        "HEAD"
      ];
      const methodsWithBody = [
        "POST",
        "PUT",
        "DELETE",
        "PATCH"
      ];
      push("val client = OkHttpClient()");
      blank();
      if (postData.text) {
        if (postData.boundary) push(`val mediaType = MediaType.parse("${postData.mimeType}; boundary=${postData.boundary}")`);
        else push(`val mediaType = MediaType.parse("${postData.mimeType}")`);
        push(`val body = RequestBody.create(mediaType, ${JSON.stringify(postData.text)})`);
      }
      push("val request = Request.Builder()");
      push(`.url("${fullUrl}")`, 1);
      if (!methods.includes(method.toUpperCase())) if (postData.text) push(`.method("${method.toUpperCase()}", body)`, 1);
      else push(`.method("${method.toUpperCase()}", null)`, 1);
      else if (methodsWithBody.includes(method.toUpperCase())) if (postData.text) push(`.${method.toLowerCase()}(body)`, 1);
      else push(`.${method.toLowerCase()}(null)`, 1);
      else push(`.${method.toLowerCase()}()`, 1);
      Object.keys(allHeaders).forEach((key) => {
        push(`.addHeader("${key}", "${escapeForDoubleQuotes(allHeaders[key])}")`, 1);
      });
      push(".build()", 1);
      blank();
      push("val response = client.newCall(request).execute()");
      return join();
    }
  } }
};
var node = {
  info: {
    key: "node",
    title: "Node.js",
    default: "fetch",
    cli: "node %s"
  },
  clientsById: {
    native: {
      info: {
        key: "native",
        title: "HTTP",
        link: "http://nodejs.org/api/http.html#http_http_request_options_callback",
        description: "Node.js native HTTP interface",
        extname: ".cjs"
      },
      convert: ({ uriObj, method, allHeaders, postData }, options = {}) => {
        const { indent = "  " } = options;
        const { blank, join, push, unshift } = new CodeBuilder({ indent });
        const reqOpts = {
          method,
          hostname: uriObj.hostname,
          port: uriObj.port,
          path: uriObj.path,
          headers: allHeaders
        };
        push(`const http = require('${uriObj.protocol?.replace(":", "")}');`);
        blank();
        push(`const options = ${(0, import_stringify_object.default)(reqOpts, { indent })};`);
        blank();
        push("const req = http.request(options, function (res) {");
        push("const chunks = [];", 1);
        blank();
        push("res.on('data', function (chunk) {", 1);
        push("chunks.push(chunk);", 2);
        push("});", 1);
        blank();
        push("res.on('end', function () {", 1);
        push("const body = Buffer.concat(chunks);", 2);
        push("console.log(body.toString());", 2);
        push("});", 1);
        push("});");
        blank();
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            if (postData.paramsObj) {
              unshift("const qs = require('querystring');");
              push(`req.write(qs.stringify(${(0, import_stringify_object.default)(postData.paramsObj, {
                indent: "  ",
                inlineCharacterLimit: 80
              })}));`);
            }
            break;
          case "application/json":
            if (postData.jsonObj) push(`req.write(JSON.stringify(${(0, import_stringify_object.default)(postData.jsonObj, {
              indent: "  ",
              inlineCharacterLimit: 80
            })}));`);
            break;
          default:
            if (postData.text) push(`req.write(${(0, import_stringify_object.default)(postData.text, { indent })});`);
        }
        push("req.end();");
        return join();
      }
    },
    axios: {
      info: {
        key: "axios",
        title: "Axios",
        link: "https://github.com/axios/axios",
        description: "Promise based HTTP client for the browser and node.js",
        extname: ".js",
        installation: () => "npm install axios --save"
      },
      convert: ({ method, fullUrl, allHeaders, postData }, options) => {
        const { blank, join, push, addPostProcessor } = new CodeBuilder({ indent: {
          indent: "  ",
          ...options
        }.indent });
        push("import axios from 'axios';");
        blank();
        const reqOpts = {
          method,
          url: fullUrl
        };
        if (Object.keys(allHeaders).length) reqOpts.headers = allHeaders;
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            if (postData.params) {
              push("const encodedParams = new URLSearchParams();");
              postData.params.forEach((param) => {
                push(`encodedParams.set('${param.name}', '${param.value}');`);
              });
              blank();
              reqOpts.data = "encodedParams,";
              addPostProcessor((code) => code.replace(/'encodedParams,'/, "encodedParams,"));
            }
            break;
          case "application/json":
            if (postData.jsonObj) reqOpts.data = postData.jsonObj;
            break;
          default:
            if (postData.text) reqOpts.data = postData.text;
        }
        push(`const options = ${(0, import_stringify_object.default)(reqOpts, {
          indent: "  ",
          inlineCharacterLimit: 80
        })};`);
        blank();
        push("axios");
        push(".request(options)", 1);
        push(".then(res => console.log(res.data))", 1);
        push(".catch(err => console.error(err));", 1);
        return join();
      }
    },
    fetch: {
      info: {
        key: "fetch",
        title: "fetch",
        link: "https://nodejs.org/docs/latest/api/globals.html#fetch",
        description: "Perform asynchronous HTTP requests with the Fetch API",
        extname: ".js"
      },
      convert: ({ method, fullUrl, postData, headersObj, cookies }, options) => {
        const opts = {
          indent: "  ",
          ...options
        };
        let includeFS = false;
        const { blank, push, join, unshift } = new CodeBuilder({ indent: opts.indent });
        const url = fullUrl;
        const reqOpts = { method };
        if (Object.keys(headersObj).length) reqOpts.headers = headersObj;
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            push("const encodedParams = new URLSearchParams();");
            postData.params?.forEach((param) => {
              push(`encodedParams.set('${param.name}', '${param.value}');`);
            });
            reqOpts.body = "encodedParams";
            blank();
            break;
          case "application/json":
            if (postData.jsonObj) reqOpts.body = postData.jsonObj;
            break;
          case "multipart/form-data": {
            if (!postData.params) break;
            const contentTypeHeader = getHeaderName(headersObj, "content-type");
            if (contentTypeHeader) delete headersObj[contentTypeHeader];
            push("const formData = new FormData();");
            postData.params.forEach((param) => {
              if (!param.fileName && !param.contentType) {
                push(`formData.append('${param.name}', '${param.value}');`);
                return;
              }
              if (param.fileName) {
                includeFS = true;
                push(`formData.append('${param.name}', await new Response(fs.createReadStream('${param.fileName}')).blob());`);
              }
            });
            reqOpts.body = "formData";
            blank();
            break;
          }
          default:
            if (postData.text) reqOpts.body = postData.text;
        }
        if (cookies.length) {
          const cookiesString = cookies.map(({ name, value }) => `${encodeURIComponent(name)}=${encodeURIComponent(value)}`).join("; ");
          if (reqOpts.headers) reqOpts.headers.cookie = cookiesString;
          else {
            reqOpts.headers = {};
            reqOpts.headers.cookie = cookiesString;
          }
        }
        push(`const url = '${url}';`);
        if (reqOpts.headers && !Object.keys(reqOpts.headers).length) delete reqOpts.headers;
        push(`const options = ${(0, import_stringify_object.default)(reqOpts, {
          indent: "  ",
          inlineCharacterLimit: 80,
          transform: (object, property, originalResult) => {
            if (property === "body" && postData.mimeType === "application/json") return `JSON.stringify(${originalResult})`;
            return originalResult;
          }
        })};`);
        blank();
        if (includeFS) unshift("import fs from 'fs';\n");
        push("fetch(url, options)");
        push(".then(res => res.json())", 1);
        push(".then(json => console.log(json))", 1);
        push(".catch(err => console.error(err));", 1);
        return join().replace(/'encodedParams'/, "encodedParams").replace(/'formData'/, "formData");
      }
    }
  }
};
var nsDeclaration = (nsClass, name, parameters, indent) => {
  const opening = `${nsClass} *${name} = `;
  return `${opening}${literalRepresentation$3(parameters, indent ? opening.length : void 0)};`;
};
var literalRepresentation$3 = (value, indentation) => {
  const join = indentation === void 0 ? ", " : `,
   ${" ".repeat(indentation)}`;
  switch (Object.prototype.toString.call(value)) {
    case "[object Number]":
      return `@${value}`;
    case "[object Array]":
      return `@[ ${value.map((val) => literalRepresentation$3(val)).join(join)} ]`;
    case "[object Object]": {
      const keyValuePairs = [];
      Object.keys(value).forEach((key) => {
        keyValuePairs.push(`@"${key}": ${literalRepresentation$3(value[key])}`);
      });
      return `@{ ${keyValuePairs.join(join)} }`;
    }
    case "[object Boolean]":
      return value ? "@YES" : "@NO";
    default:
      if (value === null || value === void 0) return "";
      return `@"${value.toString().replace(/"/g, '\\"')}"`;
  }
};
var objc = {
  info: {
    key: "objc",
    title: "Objective-C",
    default: "nsurlsession"
  },
  clientsById: { nsurlsession: {
    info: {
      key: "nsurlsession",
      title: "NSURLSession",
      link: "https://developer.apple.com/library/mac/documentation/Foundation/Reference/NSURLSession_class/index.html",
      description: "Foundation's NSURLSession request",
      extname: ".m"
    },
    convert: ({ allHeaders, postData, method, fullUrl }, options) => {
      const opts = {
        indent: "    ",
        pretty: true,
        timeout: 10,
        ...options
      };
      const { push, join, blank } = new CodeBuilder({ indent: opts.indent });
      const req = {
        hasHeaders: false,
        hasBody: false
      };
      push("#import <Foundation/Foundation.h>");
      if (Object.keys(allHeaders).length) {
        req.hasHeaders = true;
        blank();
        push(nsDeclaration("NSDictionary", "headers", allHeaders, opts.pretty));
      }
      if (postData.text || postData.jsonObj || postData.params) {
        req.hasBody = true;
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            if (postData.params?.length) {
              blank();
              const [head, ...tail] = postData.params;
              push(`NSMutableData *postData = [[NSMutableData alloc] initWithData:[@"${head.name}=${head.value}" dataUsingEncoding:NSUTF8StringEncoding]];`);
              tail.forEach(({ name, value }) => {
                push(`[postData appendData:[@"&${name}=${value}" dataUsingEncoding:NSUTF8StringEncoding]];`);
              });
            } else req.hasBody = false;
            break;
          case "application/json":
            if (postData.jsonObj) {
              push(nsDeclaration("NSDictionary", "parameters", postData.jsonObj, opts.pretty));
              blank();
              push("NSData *postData = [NSJSONSerialization dataWithJSONObject:parameters options:0 error:nil];");
            }
            break;
          case "multipart/form-data":
            push(nsDeclaration("NSArray", "parameters", postData.params || [], opts.pretty));
            push(`NSString *boundary = @"${postData.boundary}";`);
            blank();
            push("NSError *error;");
            push("NSMutableString *body = [NSMutableString string];");
            push("for (NSDictionary *param in parameters) {");
            push('[body appendFormat:@"--%@\\r\\n", boundary];', 1);
            push('if (param[@"fileName"]) {', 1);
            push('[body appendFormat:@"Content-Disposition:form-data; name=\\"%@\\"; filename=\\"%@\\"\\r\\n", param[@"name"], param[@"fileName"]];', 2);
            push('[body appendFormat:@"Content-Type: %@\\r\\n\\r\\n", param[@"contentType"]];', 2);
            push('[body appendFormat:@"%@", [NSString stringWithContentsOfFile:param[@"fileName"] encoding:NSUTF8StringEncoding error:&error]];', 2);
            push("if (error) {", 2);
            push('NSLog(@"%@", error);', 3);
            push("}", 2);
            push("} else {", 1);
            push('[body appendFormat:@"Content-Disposition:form-data; name=\\"%@\\"\\r\\n\\r\\n", param[@"name"]];', 2);
            push('[body appendFormat:@"%@", param[@"value"]];', 2);
            push("}", 1);
            push("}");
            push('[body appendFormat:@"\\r\\n--%@--\\r\\n", boundary];');
            push("NSData *postData = [body dataUsingEncoding:NSUTF8StringEncoding];");
            break;
          default:
            blank();
            push(`NSData *postData = [[NSData alloc] initWithData:[@"${postData.text}" dataUsingEncoding:NSUTF8StringEncoding]];`);
        }
      }
      blank();
      push(`NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:[NSURL URLWithString:@"${fullUrl}"]`);
      push("                                                       cachePolicy:NSURLRequestUseProtocolCachePolicy");
      push(`                                                   timeoutInterval:${opts.timeout.toFixed(1)}];`);
      push(`[request setHTTPMethod:@"${method}"];`);
      if (req.hasHeaders) push("[request setAllHTTPHeaderFields:headers];");
      if (req.hasBody) push("[request setHTTPBody:postData];");
      blank();
      push("NSURLSession *session = [NSURLSession sharedSession];");
      push("NSURLSessionDataTask *dataTask = [session dataTaskWithRequest:request");
      push("                                            completionHandler:^(NSData *data, NSURLResponse *response, NSError *error) {");
      push("                                            if (error) {", 1);
      push('                                            NSLog(@"%@", error);', 2);
      push("                                            } else {", 1);
      push("                                            NSHTTPURLResponse *httpResponse = (NSHTTPURLResponse *) response;", 2);
      push('                                            NSLog(@"%@", httpResponse);', 2);
      push("                                            }", 1);
      push("                                            }];");
      push("[dataTask resume];");
      return join();
    }
  } }
};
var ocaml = {
  info: {
    key: "ocaml",
    title: "OCaml",
    default: "cohttp"
  },
  clientsById: { cohttp: {
    info: {
      key: "cohttp",
      title: "CoHTTP",
      link: "https://github.com/mirage/ocaml-cohttp",
      description: "Cohttp is a very lightweight HTTP server using Lwt or Async for OCaml",
      extname: ".ml",
      installation: () => "opam install cohttp-lwt-unix cohttp-async"
    },
    convert: ({ fullUrl, allHeaders, postData, method }, options) => {
      const opts = {
        indent: "  ",
        ...options
      };
      const methods = [
        "get",
        "post",
        "head",
        "delete",
        "patch",
        "put",
        "options"
      ];
      const { push, blank, join } = new CodeBuilder({ indent: opts.indent });
      push("open Cohttp_lwt_unix");
      push("open Cohttp");
      push("open Lwt");
      blank();
      push(`let uri = Uri.of_string "${fullUrl}" in`);
      const headers2 = Object.keys(allHeaders);
      if (headers2.length === 1) push(`let headers = Header.add (Header.init ()) "${headers2[0]}" "${escapeForDoubleQuotes(allHeaders[headers2[0]])}" in`);
      else if (headers2.length > 1) {
        push("let headers = Header.add_list (Header.init ()) [");
        headers2.forEach((key) => {
          push(`("${key}", "${escapeForDoubleQuotes(allHeaders[key])}");`, 1);
        });
        push("] in");
      }
      if (postData.text) push(`let body = Cohttp_lwt_body.of_string ${JSON.stringify(postData.text)} in`);
      blank();
      push(`Client.call ${headers2.length ? "~headers " : ""}${postData.text ? "~body " : ""}${methods.includes(method.toLowerCase()) ? `\`${method.toUpperCase()}` : `(Code.method_of_string "${method}")`} uri`);
      push(">>= fun (res, body_stream) ->");
      push("(* Do stuff with the result *)", 1);
      return join();
    }
  } }
};
var phpEscapes = {
  "\\": "\\\\",
  '"': '\\"',
  "$": "\\$",
  "	": "\\t",
  "\n": "\\n",
  "\v": "\\v",
  "\f": "\\f",
  "\r": "\\r",
  "\x1B": "\\e"
};
var phpString = (value) => {
  if (!/[\u0000-\u0009\u000b-\u001f\u007f]/.test(value)) return `'${escapeString(value, {
    delimiter: "'",
    escapeNewlines: false
  })}'`;
  return `"${value.replace(/[\\"$\u0000-\u001f\u007f]/g, (c2) => phpEscapes[c2] ?? `\\x${c2.charCodeAt(0).toString(16).padStart(2, "0")}`)}"`;
};
var convertType = (obj, indent, lastIndent) => {
  lastIndent = lastIndent || "";
  indent = indent || "";
  switch (Object.prototype.toString.call(obj)) {
    case "[object Boolean]":
      return obj;
    case "[object Null]":
      return "null";
    case "[object Undefined]":
      return "null";
    case "[object String]":
      return phpString(obj);
    case "[object Number]":
      return obj.toString();
    case "[object Array]": {
      const contents = obj.map((item) => convertType(item, `${indent}${indent}`, indent)).join(`,
${indent}`);
      return `[
${indent}${contents}
${lastIndent}]`;
    }
    case "[object Object]": {
      const result = [];
      for (const i in obj) if (Object.prototype.hasOwnProperty.call(obj, i)) result.push(`${convertType(i, indent)} => ${convertType(obj[i], `${indent}${indent}`, indent)}`);
      return `[
${indent}${result.join(`,
${indent}`)}
${lastIndent}]`;
    }
    default:
      return "null";
  }
};
var supportedMethods = [
  "ACL",
  "BASELINE_CONTROL",
  "CHECKIN",
  "CHECKOUT",
  "CONNECT",
  "COPY",
  "DELETE",
  "GET",
  "HEAD",
  "LABEL",
  "LOCK",
  "MERGE",
  "MKACTIVITY",
  "MKCOL",
  "MKWORKSPACE",
  "MOVE",
  "OPTIONS",
  "POST",
  "PROPFIND",
  "PROPPATCH",
  "PUT",
  "REPORT",
  "TRACE",
  "UNCHECKOUT",
  "UNLOCK",
  "UPDATE",
  "VERSION_CONTROL"
];
var php = {
  info: {
    key: "php",
    title: "PHP",
    default: "curl",
    cli: "php %s"
  },
  clientsById: {
    curl: {
      info: {
        key: "curl",
        title: "cURL",
        link: "http://php.net/manual/en/book.curl.php",
        description: "PHP with ext-curl",
        extname: ".php"
      },
      convert: ({ uriObj, postData, fullUrl, method, httpVersion, cookies, headersObj }, options = {}) => {
        const { closingTag = false, indent = "  ", maxRedirects = 10, namedErrors = false, noTags = false, shortTags = false, timeout = 30 } = options;
        const { push, blank, join } = new CodeBuilder({ indent });
        if (!noTags) {
          push(shortTags ? "<?" : "<?php");
          blank();
        }
        push("$curl = curl_init();");
        blank();
        const curlOptions = [
          {
            escape: true,
            name: "CURLOPT_PORT",
            value: uriObj.port
          },
          {
            escape: true,
            name: "CURLOPT_URL",
            value: fullUrl
          },
          {
            escape: false,
            name: "CURLOPT_RETURNTRANSFER",
            value: "true"
          },
          {
            escape: true,
            name: "CURLOPT_ENCODING",
            value: ""
          },
          {
            escape: false,
            name: "CURLOPT_MAXREDIRS",
            value: maxRedirects
          },
          {
            escape: false,
            name: "CURLOPT_TIMEOUT",
            value: timeout
          },
          {
            escape: false,
            name: "CURLOPT_HTTP_VERSION",
            value: httpVersion === "HTTP/1.0" ? "CURL_HTTP_VERSION_1_0" : "CURL_HTTP_VERSION_1_1"
          },
          {
            escape: true,
            name: "CURLOPT_CUSTOMREQUEST",
            value: method
          },
          {
            escape: !postData.jsonObj,
            name: "CURLOPT_POSTFIELDS",
            value: postData ? postData.jsonObj ? `json_encode(${convertType(postData.jsonObj, indent.repeat(2), indent)})` : postData.text : void 0
          }
        ];
        push("curl_setopt_array($curl, [");
        const curlopts = new CodeBuilder({
          indent,
          join: `
${indent}`
        });
        curlOptions.forEach(({ value, name, escape: escape3 }) => {
          if (value !== null && value !== void 0) curlopts.push(`${name} => ${escape3 ? JSON.stringify(value) : value},`);
        });
        const curlCookies = cookies.map((cookie) => `${encodeURIComponent(cookie.name)}=${encodeURIComponent(cookie.value)}`);
        if (curlCookies.length) curlopts.push(`CURLOPT_COOKIE => "${curlCookies.join("; ")}",`);
        const headers2 = Object.keys(headersObj).sort().map((key) => `"${key}: ${escapeForDoubleQuotes(headersObj[key])}"`);
        if (headers2.length) {
          curlopts.push("CURLOPT_HTTPHEADER => [");
          curlopts.push(headers2.join(`,
${indent}${indent}`), 1);
          curlopts.push("],");
        }
        push(curlopts.join(), 1);
        push("]);");
        blank();
        push("$response = curl_exec($curl);");
        push("$err = curl_error($curl);");
        blank();
        push("curl_close($curl);");
        blank();
        push("if ($err) {");
        if (namedErrors) push('echo array_flip(get_defined_constants(true)["curl"])[$err];', 1);
        else push('echo "cURL Error #:" . $err;', 1);
        push("} else {");
        push("echo $response;", 1);
        push("}");
        if (!noTags && closingTag) {
          blank();
          push("?>");
        }
        return join();
      }
    },
    guzzle: {
      info: {
        key: "guzzle",
        title: "Guzzle",
        link: "http://docs.guzzlephp.org/en/stable/",
        description: "PHP with Guzzle",
        extname: ".php",
        installation: () => "composer require guzzlehttp/guzzle"
      },
      convert: ({ postData, fullUrl, method, cookies, headersObj }, options) => {
        const opts = {
          closingTag: false,
          indent: "  ",
          noTags: false,
          shortTags: false,
          ...options
        };
        const { push, blank, join } = new CodeBuilder({ indent: opts.indent });
        const { code: requestCode, push: requestPush, join: requestJoin } = new CodeBuilder({ indent: opts.indent });
        if (!opts.noTags) push(opts.shortTags ? "<?" : "<?php");
        push("require_once('vendor/autoload.php');");
        blank();
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            requestPush(`'form_params' => ${convertType(postData.paramsObj, opts.indent + opts.indent, opts.indent)},`, 1);
            break;
          case "multipart/form-data": {
            const fields = [];
            if (postData.params) postData.params.forEach((param) => {
              if (param.fileName) {
                const field = {
                  name: param.name,
                  filename: param.fileName,
                  contents: param.value
                };
                if (param.contentType) field.headers = { "Content-Type": param.contentType };
                fields.push(field);
              } else fields.push({
                name: param.name,
                contents: param.value || ""
              });
            });
            if (fields.length) {
              requestPush(`'multipart' => ${convertType(fields, opts.indent + opts.indent, opts.indent)},`, 1);
              if (hasHeader(headersObj, "content-type")) {
                if (getHeader(headersObj, "content-type")?.indexOf("boundary")) {
                  const headerName = getHeaderName(headersObj, "content-type");
                  if (headerName) delete headersObj[headerName];
                }
              }
            }
            break;
          }
          default:
            if (postData.text) requestPush(`'body' => ${convertType(postData.text)},`, 1);
        }
        const headers2 = Object.keys(headersObj).sort().map((key) => `${opts.indent}${opts.indent}${convertType(key)} => ${convertType(headersObj[key])},`);
        const cookieString = cookies.map((cookie) => `${encodeURIComponent(cookie.name)}=${encodeURIComponent(cookie.value)}`).join("; ");
        if (cookieString.length) headers2.push(`${opts.indent}${opts.indent}'cookie' => ${convertType(cookieString)},`);
        if (headers2.length) {
          requestPush("'headers' => [", 1);
          requestPush(headers2.join("\n"));
          requestPush("],", 1);
        }
        push("$client = new \\GuzzleHttp\\Client();");
        blank();
        if (requestCode.length) {
          push(`$response = $client->request(${convertType(method)}, ${convertType(fullUrl)}, [`);
          push(requestJoin());
          push("]);");
        } else push(`$response = $client->request(${convertType(method)}, ${convertType(fullUrl)});`);
        blank();
        push("echo $response->getBody();");
        if (!opts.noTags && opts.closingTag) {
          blank();
          push("?>");
        }
        return join();
      }
    },
    http1: {
      info: {
        key: "http1",
        title: "HTTP v1",
        link: "http://php.net/manual/en/book.http.php",
        description: "PHP with pecl/http v1",
        extname: ".php"
      },
      convert: ({ method, url, postData, queryObj, headersObj, cookiesObj }, options = {}) => {
        const { closingTag = false, indent = "  ", noTags = false, shortTags = false } = options;
        const { push, blank, join } = new CodeBuilder({ indent });
        if (!noTags) {
          push(shortTags ? "<?" : "<?php");
          blank();
        }
        if (!supportedMethods.includes(method.toUpperCase())) push(`HttpRequest::methodRegister('${method}');`);
        push("$request = new HttpRequest();");
        push(`$request->setUrl(${convertType(url)});`);
        if (supportedMethods.includes(method.toUpperCase())) push(`$request->setMethod(HTTP_METH_${method.toUpperCase()});`);
        else push(`$request->setMethod(HttpRequest::HTTP_METH_${method.toUpperCase()});`);
        blank();
        if (Object.keys(queryObj).length) {
          push(`$request->setQueryData(${convertType(queryObj, indent)});`);
          blank();
        }
        if (Object.keys(headersObj).length) {
          push(`$request->setHeaders(${convertType(headersObj, indent)});`);
          blank();
        }
        if (Object.keys(cookiesObj).length) {
          push(`$request->setCookies(${convertType(cookiesObj, indent)});`);
          blank();
        }
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            push(`$request->setContentType(${convertType(postData.mimeType)});`);
            push(`$request->setPostFields(${convertType(postData.paramsObj, indent)});`);
            blank();
            break;
          case "application/json":
            push(`$request->setContentType(${convertType(postData.mimeType)});`);
            push(`$request->setBody(json_encode(${convertType(postData.jsonObj, indent)}));`);
            blank();
            break;
          default:
            if (postData.text) {
              push(`$request->setBody(${convertType(postData.text)});`);
              blank();
            }
        }
        push("try {");
        push("$response = $request->send();", 1);
        blank();
        push("echo $response->getBody();", 1);
        push("} catch (HttpException $ex) {");
        push("echo $ex;", 1);
        push("}");
        if (!noTags && closingTag) {
          blank();
          push("?>");
        }
        return join();
      }
    },
    http2: {
      info: {
        key: "http2",
        title: "HTTP v2",
        link: "http://devel-m6w6.rhcloud.com/mdref/http",
        description: "PHP with pecl/http v2",
        extname: ".php"
      },
      convert: ({ postData, headersObj, method, queryObj, cookiesObj, url }, options = {}) => {
        const { closingTag = false, indent = "  ", noTags = false, shortTags = false } = options;
        const { push, blank, join } = new CodeBuilder({ indent });
        let hasBody = false;
        if (!noTags) {
          push(shortTags ? "<?" : "<?php");
          blank();
        }
        push("$client = new http\\Client;");
        push("$request = new http\\Client\\Request;");
        blank();
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            push("$body = new http\\Message\\Body;");
            push(`$body->append(new http\\QueryString(${convertType(postData.paramsObj, indent)}));`);
            blank();
            hasBody = true;
            break;
          case "multipart/form-data": {
            if (!postData.params) break;
            const files = [];
            const fields = {};
            postData.params.forEach(({ name, fileName, value, contentType }) => {
              if (fileName) {
                files.push({
                  name,
                  type: contentType,
                  file: fileName,
                  data: value
                });
                return;
              }
              if (value) fields[name] = value;
            });
            const field = Object.keys(fields).length ? convertType(fields, indent) : "null";
            const formValue = files.length ? convertType(files, indent) : "null";
            push("$body = new http\\Message\\Body;");
            push(`$body->addForm(${field}, ${formValue});`);
            if (hasHeader(headersObj, "content-type")) {
              if (getHeader(headersObj, "content-type")?.indexOf("boundary")) {
                const headerName = getHeaderName(headersObj, "content-type");
                if (headerName) delete headersObj[headerName];
              }
            }
            blank();
            hasBody = true;
            break;
          }
          case "application/json":
            push("$body = new http\\Message\\Body;");
            push(`$body->append(json_encode(${convertType(postData.jsonObj, indent)}));`);
            hasBody = true;
            break;
          default:
            if (postData.text) {
              push("$body = new http\\Message\\Body;");
              push(`$body->append(${convertType(postData.text)});`);
              blank();
              hasBody = true;
            }
        }
        push(`$request->setRequestUrl(${convertType(url)});`);
        push(`$request->setRequestMethod(${convertType(method)});`);
        if (hasBody) {
          push("$request->setBody($body);");
          blank();
        }
        if (Object.keys(queryObj).length) {
          push(`$request->setQuery(new http\\QueryString(${convertType(queryObj, indent)}));`);
          blank();
        }
        if (Object.keys(headersObj).length) {
          push(`$request->setHeaders(${convertType(headersObj, indent)});`);
          blank();
        }
        if (Object.keys(cookiesObj).length) {
          blank();
          push(`$client->setCookies(${convertType(cookiesObj, indent)});`);
          blank();
        }
        push("$client->enqueue($request)->send();");
        push("$response = $client->getResponse();");
        blank();
        push("echo $response->getBody();");
        if (!noTags && closingTag) {
          blank();
          push("?>");
        }
        return join();
      }
    }
  }
};
var generatePowershellConvert = (command) => {
  const convert2 = ({ method, headersObj, cookies, uriObj, fullUrl, postData, allHeaders }) => {
    const { push, join } = new CodeBuilder();
    if (![
      "GET",
      "POST",
      "PUT",
      "DELETE",
      "PATCH",
      "HEAD",
      "OPTIONS"
    ].includes(method.toUpperCase())) return "Method not supported";
    const commandOptions = [];
    const headers2 = Object.keys(headersObj);
    if (headers2.length) {
      push("$headers=@{}");
      headers2.forEach((key) => {
        if (key !== "connection") push(`$headers.Add("${key}", "${escapeString(headersObj[key], { escapeChar: "`" })}")`);
      });
      commandOptions.push("-Headers $headers");
    }
    if (cookies.length) {
      push("$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession");
      cookies.forEach((cookie) => {
        push("$cookie = New-Object System.Net.Cookie");
        push(`$cookie.Name = '${cookie.name}'`);
        push(`$cookie.Value = '${cookie.value}'`);
        push(`$cookie.Domain = '${uriObj.host}'`);
        push("$session.Cookies.Add($cookie)");
      });
      commandOptions.push("-WebSession $session");
    }
    if (postData.text) {
      commandOptions.push(`-ContentType '${escapeString(getHeader(allHeaders, "content-type"), {
        delimiter: "'",
        escapeChar: "`"
      })}'`);
      commandOptions.push(`-Body '${postData.text}'`);
    }
    push(`$response = ${command} -Uri '${fullUrl}' -Method ${method} ${commandOptions.join(" ")}`.trim());
    return join();
  };
  return convert2;
};
var restmethod = {
  info: {
    key: "restmethod",
    title: "Invoke-RestMethod",
    link: "https://docs.microsoft.com/en-us/powershell/module/Microsoft.PowerShell.Utility/Invoke-RestMethod",
    description: "Powershell Invoke-RestMethod client",
    extname: ".ps1"
  },
  convert: generatePowershellConvert("Invoke-RestMethod")
};
var powershell = {
  info: {
    key: "powershell",
    title: "Powershell",
    default: "webrequest"
  },
  clientsById: {
    webrequest: {
      info: {
        key: "webrequest",
        title: "Invoke-WebRequest",
        link: "https://docs.microsoft.com/en-us/powershell/module/Microsoft.PowerShell.Utility/Invoke-WebRequest",
        description: "Powershell Invoke-WebRequest client",
        extname: ".ps1"
      },
      convert: generatePowershellConvert("Invoke-WebRequest")
    },
    restmethod
  }
};
function concatValues$1(concatType, values, pretty, indentation, indentLevel) {
  const currentIndent = indentation.repeat(indentLevel);
  const closingBraceIndent = indentation.repeat(indentLevel - 1);
  const join = pretty ? `,
${currentIndent}` : ", ";
  const openingBrace = concatType === "object" ? "{" : "[";
  const closingBrace = concatType === "object" ? "}" : "]";
  if (pretty) return `${openingBrace}
${currentIndent}${values.join(join)}
${closingBraceIndent}${closingBrace}`;
  if (concatType === "object" && values.length > 0) return `${openingBrace} ${values.join(join)} ${closingBrace}`;
  return `${openingBrace}${values.join(join)}${closingBrace}`;
}
var literalRepresentation$2 = (value, opts, indentLevel) => {
  indentLevel = indentLevel === void 0 ? 1 : indentLevel + 1;
  switch (Object.prototype.toString.call(value)) {
    case "[object Number]":
      return value;
    case "[object Array]": {
      let pretty = false;
      return concatValues$1("array", value.map((v) => {
        if (Object.prototype.toString.call(v) === "[object Object]") pretty = Object.keys(v).length > 1;
        return literalRepresentation$2(v, opts, indentLevel);
      }), pretty, opts.indent, indentLevel);
    }
    case "[object Object]": {
      const keyValuePairs = [];
      for (const key in value) keyValuePairs.push(`"${key}": ${literalRepresentation$2(value[key], opts, indentLevel)}`);
      return concatValues$1("object", keyValuePairs, opts.pretty && keyValuePairs.length > 1, opts.indent, indentLevel);
    }
    case "[object Null]":
      return "None";
    case "[object Boolean]":
      return value ? "True" : "False";
    default:
      if (value === null || value === void 0) return "";
      return `"${value.toString().replace(/"/g, '\\"')}"`;
  }
};
var builtInMethods = [
  "HEAD",
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "OPTIONS"
];
var python = {
  info: {
    key: "python",
    title: "Python",
    default: "requests",
    cli: "python3 %s"
  },
  clientsById: { requests: {
    info: {
      key: "requests",
      title: "Requests",
      link: "http://docs.python-requests.org/en/latest/api/#requests.request",
      description: "Requests HTTP library",
      extname: ".py",
      installation: () => "python -m pip install requests"
    },
    convert: ({ fullUrl, postData, allHeaders, method }, options) => {
      const opts = {
        indent: "    ",
        pretty: true,
        ...options
      };
      const { push, blank, join, addPostProcessor } = new CodeBuilder({ indent: opts.indent });
      push("import requests");
      blank();
      push(`url = "${fullUrl}"`);
      blank();
      const headers2 = allHeaders;
      let payload = {};
      const files = {};
      let hasFiles = false;
      let hasPayload = false;
      let jsonPayload = false;
      switch (postData.mimeType) {
        case "application/json":
          if (postData.jsonObj) {
            push(`payload = ${literalRepresentation$2(postData.jsonObj, opts)}`);
            jsonPayload = true;
            hasPayload = true;
          }
          break;
        case "multipart/form-data":
          if (!postData.params) break;
          payload = {};
          postData.params.forEach((p) => {
            if (p.fileName) {
              if (p.contentType) files[p.name] = `('${p.fileName}', open('${p.fileName}', 'rb'), '${p.contentType}')`;
              else files[p.name] = `('${p.fileName}', open('${p.fileName}', 'rb'))`;
              hasFiles = true;
            } else {
              payload[p.name] = p.value;
              hasPayload = true;
            }
          });
          if (hasFiles) {
            push(`files = ${literalRepresentation$2(files, opts)}`);
            if (hasPayload) push(`payload = ${literalRepresentation$2(payload, opts)}`);
            const headerName = getHeaderName(headers2, "content-type");
            if (headerName) delete headers2[headerName];
          } else {
            const nonFilePayload = JSON.stringify(postData.text);
            if (nonFilePayload) {
              push(`payload = ${nonFilePayload}`);
              hasPayload = true;
            }
          }
          addPostProcessor((code) => code.replace(/"\('(.+)', open\('(.+)', 'rb'\)\)"/g, '("$1", open("$2", "rb"))').replace(/"\('(.+)', open\('(.+)', 'rb'\), '(.+)'\)"/g, '("$1", open("$2", "rb"), "$3")'));
          break;
        default: {
          if (postData.mimeType === "application/x-www-form-urlencoded" && postData.paramsObj) {
            push(`payload = ${literalRepresentation$2(postData.paramsObj, opts)}`);
            hasPayload = true;
            break;
          }
          const stringPayload = JSON.stringify(postData.text);
          if (stringPayload) {
            push(`payload = ${stringPayload}`);
            hasPayload = true;
          }
        }
      }
      const headerCount = Object.keys(headers2).length;
      if (headerCount === 0 && (hasPayload || hasFiles)) blank();
      else if (headerCount === 1) Object.keys(headers2).forEach((header) => {
        push(`headers = {"${header}": "${escapeForDoubleQuotes(headers2[header])}"}`);
        blank();
      });
      else if (headerCount > 1) {
        let count = 1;
        push("headers = {");
        Object.keys(headers2).forEach((header) => {
          if (count !== headerCount) push(`"${header}": "${escapeForDoubleQuotes(headers2[header])}",`, 1);
          else push(`"${header}": "${escapeForDoubleQuotes(headers2[header])}"`, 1);
          count += 1;
        });
        push("}");
        blank();
      }
      let request = builtInMethods.includes(method) ? `response = requests.${method.toLowerCase()}(url` : `response = requests.request("${method}", url`;
      if (hasPayload) if (jsonPayload) request += ", json=payload";
      else request += ", data=payload";
      if (hasFiles) request += ", files=files";
      if (headerCount > 0) request += ", headers=headers";
      request += ")";
      push(request);
      blank();
      push("print(response.text)");
      return join();
    }
  } }
};
var r = {
  info: {
    key: "r",
    title: "R",
    default: "httr"
  },
  clientsById: { httr: {
    info: {
      key: "httr",
      title: "httr",
      link: "https://cran.r-project.org/web/packages/httr/vignettes/quickstart.html",
      description: "httr: Tools for Working with URLs and HTTP",
      extname: ".r"
    },
    convert: ({ url, queryObj, queryString, postData, allHeaders, method }) => {
      const { push, blank, join } = new CodeBuilder();
      push("library(httr)");
      blank();
      push(`url <- "${url}"`);
      blank();
      const qs = queryObj;
      delete queryObj.key;
      const queryCount = Object.keys(qs).length;
      if (queryString.length === 1) {
        push(`queryString <- list(${Object.keys(qs)} = "${Object.values(qs).toString()}")`);
        blank();
      } else if (queryString.length > 1) {
        push("queryString <- list(");
        Object.keys(qs).forEach((query, i) => {
          if (i !== queryCount - 1) push(`  ${query} = "${qs[query].toString()}",`);
          else push(`  ${query} = "${qs[query].toString()}"`);
        });
        push(")");
        blank();
      }
      const payload = JSON.stringify(postData.text);
      if (payload) {
        push(`payload <- ${payload}`);
        blank();
      }
      if (postData.text || postData.jsonObj || postData.params) switch (postData.mimeType) {
        case "application/x-www-form-urlencoded":
          push('encode <- "form"');
          blank();
          break;
        case "application/json":
          push('encode <- "json"');
          blank();
          break;
        case "multipart/form-data":
          push('encode <- "multipart"');
          blank();
          break;
        default:
          push('encode <- "raw"');
          blank();
          break;
      }
      const cookieHeader = getHeader(allHeaders, "cookie");
      const acceptHeader = getHeader(allHeaders, "accept");
      const setCookies = cookieHeader ? `set_cookies(\`${String(cookieHeader).replace(/;/g, '", `').replace(/` /g, "`").replace(/[=]/g, '` = "')}")` : void 0;
      const setAccept = acceptHeader ? `accept("${escapeForDoubleQuotes(acceptHeader)}")` : void 0;
      const setContentType = `content_type("${escapeForDoubleQuotes(postData.mimeType)}")`;
      const otherHeaders = Object.entries(allHeaders).filter(([key]) => ![
        "cookie",
        "accept",
        "content-type"
      ].includes(key.toLowerCase())).map(([key, value]) => `'${key}' = '${escapeForSingleQuotes(value)}'`).join(", ");
      const setHeaders = otherHeaders ? `add_headers(${otherHeaders})` : void 0;
      let request = `response <- VERB("${method}", url`;
      if (payload) request += ", body = payload";
      if (queryString.length) request += ", query = queryString";
      const headerAdditions = [
        setHeaders,
        setContentType,
        setAccept,
        setCookies
      ].filter((x) => !!x).join(", ");
      if (headerAdditions) request += `, ${headerAdditions}`;
      if (postData.text || postData.jsonObj || postData.params) request += ", encode = encode";
      request += ")";
      push(request);
      blank();
      push('content(response, "text")');
      return join();
    }
  } }
};
var ruby = {
  info: {
    key: "ruby",
    title: "Ruby",
    default: "native"
  },
  clientsById: {
    native: {
      info: {
        key: "native",
        title: "net::http",
        link: "http://ruby-doc.org/stdlib-2.2.1/libdoc/net/http/rdoc/Net/HTTP.html",
        description: "Ruby HTTP client",
        extname: ".rb"
      },
      convert: ({ uriObj, method: rawMethod, fullUrl, postData, allHeaders }) => {
        const { push, blank, join } = new CodeBuilder();
        push("require 'uri'");
        push("require 'net/http'");
        blank();
        const method = rawMethod.toUpperCase();
        const methods = [
          "GET",
          "POST",
          "HEAD",
          "DELETE",
          "PATCH",
          "PUT",
          "OPTIONS",
          "COPY",
          "LOCK",
          "UNLOCK",
          "MOVE",
          "TRACE"
        ];
        const capMethod = method.charAt(0) + method.substring(1).toLowerCase();
        if (!methods.includes(method)) {
          push(`class Net::HTTP::${capMethod} < Net::HTTPRequest`);
          push(`  METHOD = '${method.toUpperCase()}'`);
          push(`  REQUEST_HAS_BODY = '${postData.text ? "true" : "false"}'`);
          push("  RESPONSE_HAS_BODY = true");
          push("end");
          blank();
        }
        push(`url = URI("${fullUrl}")`);
        blank();
        push("http = Net::HTTP.new(url.host, url.port)");
        if (uriObj.protocol === "https:") push("http.use_ssl = true");
        blank();
        push(`request = Net::HTTP::${capMethod}.new(url)`);
        const headers2 = Object.keys(allHeaders);
        if (headers2.length) headers2.forEach((key) => {
          push(`request["${key}"] = '${escapeForSingleQuotes(allHeaders[key])}'`);
        });
        if (postData.text) push(`request.body = ${JSON.stringify(postData.text)}`);
        blank();
        push("response = http.request(request)");
        push("puts response.read_body");
        return join();
      }
    },
    faraday: {
      info: {
        key: "faraday",
        title: "faraday",
        link: "https://github.com/lostisland/faraday",
        description: "Faraday HTTP client",
        extname: ".rb"
      },
      convert: ({ uriObj, queryObj, method: rawMethod, postData, allHeaders }) => {
        const { push, blank, join } = new CodeBuilder();
        const method = rawMethod.toUpperCase();
        if (![
          "GET",
          "POST",
          "HEAD",
          "DELETE",
          "PATCH",
          "PUT",
          "OPTIONS",
          "COPY",
          "LOCK",
          "UNLOCK",
          "MOVE",
          "TRACE"
        ].includes(method)) {
          push(`# Faraday cannot currently run ${method} requests. Please use another client.`);
          return join();
        }
        push("require 'faraday'");
        blank();
        if (postData.mimeType === "application/x-www-form-urlencoded") {
          if (postData.params) {
            push(`data = {`);
            postData.params.forEach((param) => {
              push(`  :${param.name} => ${JSON.stringify(param.value)},`);
            });
            push(`}`);
            blank();
          }
        }
        push(`conn = Faraday.new(`);
        push(`  url: '${uriObj.protocol}//${uriObj.host}',`);
        if (allHeaders["content-type"] || allHeaders["Content-Type"]) push(`  headers: {'Content-Type' => '${allHeaders["content-type"] || allHeaders["Content-Type"]}'}`);
        push(`)`);
        blank();
        push(`response = conn.${method.toLowerCase()}('${uriObj.pathname}') do |req|`);
        const headers2 = Object.keys(allHeaders);
        if (headers2.length) headers2.forEach((key) => {
          if (key.toLowerCase() !== "content-type") push(`  req.headers['${key}'] = '${escapeForSingleQuotes(allHeaders[key])}'`);
        });
        Object.keys(queryObj).forEach((name) => {
          const value = queryObj[name];
          if (Array.isArray(value)) push(`  req.params['${name}'] = ${JSON.stringify(value)}`);
          else push(`  req.params['${name}'] = '${value}'`);
        });
        switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            if (postData.params) push(`  req.body = URI.encode_www_form(data)`);
            break;
          case "application/json":
            if (postData.jsonObj) push(`  req.body = ${JSON.stringify(postData.text)}`);
            break;
          default:
            if (postData.text) push(`  req.body = ${JSON.stringify(postData.text)}`);
        }
        push("end");
        blank();
        push("puts response.status");
        push("puts response.body");
        return join();
      }
    }
  }
};
function concatValues(concatType, values, pretty, indentation, indentLevel) {
  const currentIndent = indentation.repeat(indentLevel);
  const closingBraceIndent = indentation.repeat(indentLevel - 1);
  const join = pretty ? `,
${currentIndent}` : ", ";
  const openingBrace = concatType === "object" ? "json!({" : "(";
  const closingBrace = concatType === "object" ? "})" : ")";
  if (pretty) return `${openingBrace}
${currentIndent}${values.join(join)}
${closingBraceIndent}${closingBrace}`;
  return `${openingBrace}${values.join(join)}${closingBrace}`;
}
var literalRepresentation$1 = (value, opts, indentLevel) => {
  indentLevel = indentLevel === void 0 ? 1 : indentLevel + 1;
  switch (Object.prototype.toString.call(value)) {
    case "[object Number]":
      return value;
    case "[object Array]": {
      if (value.length === 0) return "json!([])";
      let pretty = false;
      return concatValues("array", value.map((v) => {
        if (Object.prototype.toString.call(v) === "[object Object]") pretty = Object.keys(v).length > 1;
        return literalRepresentation$1(v, opts, indentLevel);
      }), pretty, opts.indent, indentLevel);
    }
    case "[object Object]": {
      const keyValuePairs = [];
      for (const key in value) keyValuePairs.push(`"${key}": ${literalRepresentation$1(value[key], opts, indentLevel)}`);
      return concatValues("object", keyValuePairs, opts.pretty && keyValuePairs.length > 1, opts.indent, indentLevel);
    }
    case "[object Null]":
      return "json!(null)";
    case "[object Boolean]":
      return value ? "true" : "false";
    default:
      if (value === null || value === void 0) return "";
      return `"${value.toString().replace(/"/g, '\\"')}"`;
  }
};
var reqwest = {
  info: {
    key: "reqwest",
    title: "reqwest",
    link: "https://docs.rs/reqwest/latest/reqwest/",
    description: "reqwest HTTP library",
    extname: ".rs"
  },
  convert: ({ queryObj, url, postData, allHeaders, method }, options) => {
    const opts = {
      indent: "    ",
      pretty: true,
      ...options
    };
    let indentLevel = 0;
    const { push, blank, join, pushToLast, unshift } = new CodeBuilder({ indent: opts.indent });
    push("use reqwest;", indentLevel);
    blank();
    push("#[tokio::main]", indentLevel);
    push("pub async fn main() {", indentLevel);
    indentLevel += 1;
    push(`let url = "${url}";`, indentLevel);
    blank();
    let hasQuery = false;
    if (Object.keys(queryObj).length) {
      hasQuery = true;
      push("let querystring = [", indentLevel);
      indentLevel += 1;
      for (const [key, value] of Object.entries(queryObj)) if (Array.isArray(value)) for (const v of value) push(`("${key}", "${decodeURIComponent(v)}"),`, indentLevel);
      else push(`("${key}", "${decodeURIComponent(String(value))}"),`, indentLevel);
      indentLevel -= 1;
      push("];", indentLevel);
      blank();
    }
    let payload = {};
    const files = {};
    let hasFiles = false;
    let hasForm = false;
    let hasBody = false;
    let jsonPayload = false;
    let isMultipart2 = false;
    switch (postData.mimeType) {
      case "application/json":
        if (postData.jsonObj) push(`let payload = ${literalRepresentation$1(postData.jsonObj, opts, indentLevel)};`, indentLevel);
        jsonPayload = true;
        break;
      case "multipart/form-data":
        isMultipart2 = true;
        if (!postData.params) {
          push(`let form = reqwest::multipart::Form::new()`, indentLevel);
          push(`.text("", "");`, indentLevel + 1);
          break;
        }
        payload = {};
        postData.params.forEach((p) => {
          if (p.fileName) {
            files[p.name] = p.fileName;
            hasFiles = true;
          } else payload[p.name] = p.value;
        });
        if (hasFiles) {
          for (const line of fileToPartString) push(line, indentLevel);
          blank();
        }
        push(`let form = reqwest::multipart::Form::new()`, indentLevel);
        for (const [name, fileName] of Object.entries(files)) push(`.part("${name}", file_to_part("${fileName}").await)`, indentLevel + 1);
        for (const [name, value] of Object.entries(payload)) push(`.text("${name}", "${value}")`, indentLevel + 1);
        pushToLast(";");
        break;
      default:
        if (postData.mimeType === "application/x-www-form-urlencoded" && postData.paramsObj) {
          push(`let payload = ${literalRepresentation$1(postData.paramsObj, opts, indentLevel)};`, indentLevel);
          hasForm = true;
          break;
        }
        if (postData.text) {
          push(`let payload = ${literalRepresentation$1(postData.text, opts, indentLevel)};`, indentLevel);
          hasBody = true;
          break;
        }
    }
    if (hasForm || jsonPayload) unshift(`use serde_json::json;`);
    if (hasForm || jsonPayload || hasBody || isMultipart2) blank();
    const headersToEmit = Object.entries(allHeaders).filter(([key]) => !(key.toLowerCase() === "content-type" && isMultipart2));
    push("let client = reqwest::Client::new();", indentLevel);
    switch (method) {
      case "POST":
        push(`let response = client.post(url)`, indentLevel);
        break;
      case "GET":
        push(`let response = client.get(url)`, indentLevel);
        break;
      default:
        push(`let response = client.request(reqwest::Method::from_str("${method}").unwrap(), url)`, indentLevel);
        unshift(`use std::str::FromStr;`);
        break;
    }
    if (hasQuery) push(`.query(&querystring)`, indentLevel + 1);
    if (isMultipart2) push(`.multipart(form)`, indentLevel + 1);
    for (const [key, value] of headersToEmit) push(`.header("${key}", ${literalRepresentation$1(value, opts)})`, indentLevel + 1);
    if (jsonPayload) push(`.json(&payload)`, indentLevel + 1);
    if (hasForm) push(`.form(&payload)`, indentLevel + 1);
    if (hasBody) push(`.body(payload)`, indentLevel + 1);
    push(".send()", indentLevel + 1);
    push(".await;", indentLevel + 1);
    blank();
    push("let results = response.unwrap()", indentLevel);
    push(".json::<serde_json::Value>()", indentLevel + 1);
    push(".await", indentLevel + 1);
    push(".unwrap();", indentLevel + 1);
    blank();
    push('println!("{}", results);', indentLevel);
    push("}\n");
    return join();
  }
};
var fileToPartString = [
  `async fn file_to_part(file_name: &'static str) -> reqwest::multipart::Part {`,
  `    let bytes = tokio::fs::read(file_name).await.unwrap();`,
  `    reqwest::multipart::Part::bytes(bytes)`,
  `        .file_name(file_name)`,
  `        .mime_str("text/plain").unwrap()`,
  `}`
];
var rust = {
  info: {
    key: "rust",
    title: "Rust",
    default: "reqwest",
    cli: "rust"
  },
  clientsById: { reqwest }
};
var quote = (value = "") => {
  if (/^[a-z0-9-_/.@%^=:]+$/i.test(value)) return value;
  return `'${value.replace(/'/g, "'\\''")}'`;
};
var escape2 = (value) => value.replace(/\r/g, "\\r").replace(/\n/g, "\\n");
var params = {
  "http1.0": "0",
  "url ": "",
  cookie: "b",
  data: "d",
  form: "F",
  globoff: "g",
  header: "H",
  insecure: "k",
  request: "X"
};
var getArg = (short) => (longName) => {
  if (short) {
    const shortName = params[longName];
    if (!shortName) return "";
    return `-${shortName}`;
  }
  return `--${longName}`;
};
var shell = {
  info: {
    key: "shell",
    title: "Shell",
    default: "curl",
    cli: "%s"
  },
  clientsById: {
    curl: {
      info: {
        key: "curl",
        title: "cURL",
        link: "http://curl.haxx.se/",
        description: "cURL is a command line tool and library for transferring data with URL syntax",
        extname: ".sh"
      },
      convert: ({ fullUrl, method, httpVersion, headersObj, allHeaders, postData }, options = {}) => {
        const { binary = false, globOff = false, indent = "  ", insecureSkipVerify = false, short = false } = options;
        const indentJSON = "  ";
        const { push, join } = new CodeBuilder({
          ...typeof indent === "string" ? { indent } : {},
          join: indent !== false ? ` \\
${indent}` : " "
        });
        const arg = getArg(short);
        let formattedUrl = quote(fullUrl);
        push(`curl ${arg("request")} ${method}`);
        if (globOff) {
          formattedUrl = unescape(formattedUrl);
          push(arg("globoff"));
        }
        push(`${arg("url ")}${formattedUrl}`);
        if (insecureSkipVerify) push(arg("insecure"));
        if (httpVersion === "HTTP/1.0") push(arg("http1.0"));
        if (getHeader(allHeaders, "accept-encoding")) push("--compressed");
        if (postData.mimeType === "multipart/form-data") {
          const contentTypeHeaderName = getHeaderName(headersObj, "content-type");
          if (contentTypeHeaderName) {
            const contentTypeHeader = headersObj[contentTypeHeaderName];
            if (contentTypeHeaderName && contentTypeHeader) {
              const noBoundary = String(contentTypeHeader).replace(/; boundary.+?(?=(;|$))/, "");
              headersObj[contentTypeHeaderName] = noBoundary;
              allHeaders[contentTypeHeaderName] = noBoundary;
            }
          }
        }
        Object.keys(headersObj).sort().forEach((key) => {
          const header = `${key}: ${headersObj[key]}`;
          push(`${arg("header")} ${quote(header)}`);
        });
        if (allHeaders.cookie) push(`${arg("cookie")} ${quote(allHeaders.cookie)}`);
        switch (postData.mimeType) {
          case "multipart/form-data":
            postData.params?.forEach((param) => {
              let post = "";
              if (param.fileName) post = `${param.name}='@${param.fileName}'`;
              else post = quote(`${param.name}=${param.value}`);
              push(`${arg("form")} ${post}`);
            });
            break;
          case "application/x-www-form-urlencoded":
            if (postData.params) postData.params.forEach((param) => {
              const encoded = encodeURIComponent(param.name);
              const name = encoded !== param.name ? encoded : param.name;
              push(`${binary ? "--data-binary" : "--data-urlencode"} ${quote(`${name}=${param.value}`)}`);
            });
            else push(`${binary ? "--data-binary" : arg("data")} ${quote(postData.text)}`);
            break;
          default: {
            if (!postData.text) break;
            let builtPayload = false;
            if (isMimeTypeJSON(postData.mimeType)) {
              if (postData.text.length > 20) try {
                const jsonPayload = JSON.parse(postData.text);
                builtPayload = true;
                if (postData.text.indexOf("'") > 0) push(`${binary ? "--data-binary" : arg("data")} @- <<EOF
${JSON.stringify(jsonPayload, null, indentJSON)}
EOF`);
                else push(`${binary ? "--data-binary" : arg("data")} '
${JSON.stringify(jsonPayload, null, indentJSON)}
'`);
              } catch {
              }
            }
            if (!builtPayload) push(`${binary ? "--data-binary" : arg("data")} ${quote(postData.text)}`);
          }
        }
        return join();
      }
    },
    httpie: {
      info: {
        key: "httpie",
        title: "HTTPie",
        link: "http://httpie.org/",
        description: "a CLI, cURL-like tool for humans",
        extname: ".sh",
        installation: () => "brew install httpie"
      },
      convert: ({ allHeaders, postData, queryObj, fullUrl, method, url }, options) => {
        const opts = {
          body: false,
          cert: false,
          headers: false,
          indent: "  ",
          pretty: false,
          print: false,
          queryParams: false,
          short: false,
          style: false,
          timeout: false,
          verbose: false,
          verify: false,
          ...options
        };
        const { push, join, unshift } = new CodeBuilder({
          indent: opts.indent,
          join: opts.indent !== false ? ` \\
${opts.indent}` : " "
        });
        let raw = false;
        const flags = [];
        if (opts.headers) flags.push(opts.short ? "-h" : "--headers");
        if (opts.body) flags.push(opts.short ? "-b" : "--body");
        if (opts.verbose) flags.push(opts.short ? "-v" : "--verbose");
        if (opts.print) flags.push(`${opts.short ? "-p" : "--print"}=${opts.print}`);
        if (opts.verify) flags.push(`--verify=${opts.verify}`);
        if (opts.cert) flags.push(`--cert=${opts.cert}`);
        if (opts.pretty) flags.push(`--pretty=${opts.pretty}`);
        if (opts.style) flags.push(`--style=${opts.style}`);
        if (opts.timeout) flags.push(`--timeout=${opts.timeout}`);
        if (opts.queryParams) Object.keys(queryObj).forEach((name) => {
          const value = queryObj[name];
          if (Array.isArray(value)) value.forEach((val) => {
            push(`${name}==${quote(val)}`);
          });
          else push(`${name}==${quote(value)}`);
        });
        Object.keys(allHeaders).sort().forEach((key) => {
          push(`${key}:${quote(allHeaders[key])}`);
        });
        if (postData.mimeType === "application/x-www-form-urlencoded") {
          if (postData.params?.length) {
            flags.push(opts.short ? "-f" : "--form");
            postData.params.forEach((param) => {
              push(`${param.name}=${quote(param.value)}`);
            });
          }
        } else raw = true;
        const cliFlags = flags.length ? `${flags.join(" ")} ` : "";
        url = quote(opts.queryParams ? url : fullUrl);
        unshift(`http ${cliFlags}${method} ${url}`);
        if (raw && postData.text) unshift(`echo ${quote(postData.text)} | `);
        return join();
      }
    },
    wget: {
      info: {
        key: "wget",
        title: "Wget",
        link: "https://www.gnu.org/software/wget/",
        description: "a free software package for retrieving files using HTTP, HTTPS",
        extname: ".sh"
      },
      convert: ({ method, postData, allHeaders, fullUrl }, options) => {
        const opts = {
          indent: "  ",
          short: false,
          verbose: false,
          ...options
        };
        const { push, join } = new CodeBuilder({
          ...typeof opts.indent === "string" ? { indent: opts.indent } : {},
          join: opts.indent !== false ? ` \\
${opts.indent}` : " "
        });
        if (opts.verbose) push(`wget ${opts.short ? "-v" : "--verbose"}`);
        else push(`wget ${opts.short ? "-q" : "--quiet"}`);
        push(`--method ${quote(method)}`);
        Object.keys(allHeaders).forEach((key) => {
          const header = `${key}: ${allHeaders[key]}`;
          push(`--header ${quote(header)}`);
        });
        if (postData.text) push(`--body-data ${escape2(quote(postData.text))}`);
        push(opts.short ? "-O" : "--output-document");
        push(`- ${quote(fullUrl)}`);
        return join();
      }
    }
  }
};
var buildString = (length, str3) => str3.repeat(length);
var concatArray = (arr, pretty, indentation, indentLevel) => {
  const currentIndent = buildString(indentLevel, indentation);
  const closingBraceIndent = buildString(indentLevel - 1, indentation);
  const join = pretty ? `,
${currentIndent}` : ", ";
  if (pretty) return `[
${currentIndent}${arr.join(join)}
${closingBraceIndent}]`;
  return `[${arr.join(join)}]`;
};
var literalDeclaration = (name, parameters, opts) => `let ${name} = ${literalRepresentation(parameters, opts)}`;
var literalRepresentation = (value, opts, indentLevel) => {
  indentLevel = indentLevel === void 0 ? 1 : indentLevel + 1;
  switch (Object.prototype.toString.call(value)) {
    case "[object Number]":
      return value;
    case "[object Array]": {
      let pretty = false;
      const valuesRepresentation = value.map((v) => {
        if (Object.prototype.toString.call(v) === "[object Object]") pretty = Object.keys(v).length > 1;
        return literalRepresentation(v, opts, indentLevel);
      });
      return concatArray(valuesRepresentation, pretty, opts.indent, indentLevel);
    }
    case "[object Object]": {
      const keyValuePairs = [];
      for (const key in value) keyValuePairs.push(`"${key}": ${literalRepresentation(value[key], opts, indentLevel)}`);
      return concatArray(keyValuePairs, opts.pretty && keyValuePairs.length > 1, opts.indent, indentLevel);
    }
    case "[object Boolean]":
      return value.toString();
    default:
      if (value === null || value === void 0) return "nil";
      return `"${value.toString().replace(/"/g, '\\"')}"`;
  }
};
var targets = {
  agent,
  c,
  clojure,
  crystal,
  csharp,
  go,
  http,
  java,
  javascript,
  json,
  kotlin,
  node,
  objc,
  ocaml,
  php,
  powershell,
  python,
  r,
  ruby,
  rust,
  shell,
  swift: {
    info: {
      key: "swift",
      title: "Swift",
      default: "urlsession"
    },
    clientsById: { urlsession: {
      info: {
        key: "urlsession",
        title: "URLSession",
        link: "https://developer.apple.com/documentation/foundation/urlsession",
        description: "Foundation's URLSession request",
        extname: ".swift"
      },
      convert: ({ allHeaders, postData, uriObj, queryObj, method }, options) => {
        const opts = {
          indent: "  ",
          pretty: true,
          timeout: 10,
          ...options
        };
        const { push, blank, join } = new CodeBuilder({ indent: opts.indent });
        push("import Foundation");
        blank();
        const hasBody = postData.text || postData.jsonObj || postData.params;
        if (hasBody) switch (postData.mimeType) {
          case "application/x-www-form-urlencoded":
            if (postData.params?.length) {
              const parameters = postData.params.map((p) => `"${p.name}": "${p.value}"`);
              if (opts.pretty) {
                push("let parameters = [");
                parameters.forEach((param) => {
                  push(`${param},`, 1);
                });
                push("]");
              } else push(`let parameters = [${parameters.join(", ")}]`);
              push('let joinedParameters = parameters.map { "\\($0.key)=\\($0.value)" }.joined(separator: "&")');
              push("let postData = Data(joinedParameters.utf8)");
              blank();
            }
            break;
          case "application/json":
            if (postData.jsonObj) {
              push(`${literalDeclaration("parameters", postData.jsonObj, opts)} as [String : Any?]`);
              blank();
              push("let postData = try JSONSerialization.data(withJSONObject: parameters, options: [])");
              blank();
            }
            break;
          case "multipart/form-data":
            push(literalDeclaration("parameters", postData.params, opts));
            blank();
            push(`let boundary = "${postData.boundary}"`);
            blank();
            push('var body = ""');
            push("for param in parameters {");
            push('let paramName = param["name"]!', 1);
            push('body += "--\\(boundary)\\r\\n"', 1);
            push('body += "Content-Disposition:form-data; name=\\"\\(paramName)\\""', 1);
            push('if let filename = param["fileName"] {', 1);
            push('let contentType = param["contentType"]!', 2);
            push("let fileContent = try String(contentsOfFile: filename, encoding: .utf8)", 2);
            push('body += "; filename=\\"\\(filename)\\"\\r\\n"', 2);
            push('body += "Content-Type: \\(contentType)\\r\\n\\r\\n"', 2);
            push("body += fileContent", 2);
            push('} else if let paramValue = param["value"] {', 1);
            push('body += "\\r\\n\\r\\n\\(paramValue)"', 2);
            push("}", 1);
            push("}");
            blank();
            push("let postData = Data(body.utf8)");
            blank();
            break;
          default:
            push(`let postData = Data("${postData.text}".utf8)`);
            blank();
        }
        push(`let url = URL(string: "${uriObj.href}")!`);
        const queries = queryObj ? Object.entries(queryObj) : [];
        if (queries.length < 1) push("var request = URLRequest(url: url)");
        else {
          push("var components = URLComponents(url: url, resolvingAgainstBaseURL: true)!");
          push("let queryItems: [URLQueryItem] = [");
          queries.forEach((query) => {
            const key = query[0];
            const value = query[1];
            switch (Object.prototype.toString.call(value)) {
              case "[object String]":
                push(`URLQueryItem(name: "${key}", value: "${value}"),`, 1);
                break;
              case "[object Array]":
                value.forEach((val) => {
                  push(`URLQueryItem(name: "${key}", value: "${val}"),`, 1);
                });
                break;
            }
          });
          push("]");
          push("components.queryItems = components.queryItems.map { $0 + queryItems } ?? queryItems");
          blank();
          push("var request = URLRequest(url: components.url!)");
        }
        push(`request.httpMethod = "${method}"`);
        push(`request.timeoutInterval = ${opts.timeout}`);
        if (Object.keys(allHeaders).length) push(`request.allHTTPHeaderFields = ${literalRepresentation(allHeaders, opts)}`);
        if (hasBody) push("request.httpBody = postData");
        blank();
        push("let (data, _) = try await URLSession.shared.data(for: request)");
        push("print(String(decoding: data, as: UTF8.self))");
        return join();
      }
    } }
  }
};

// node_modules/@readme/httpsnippet/dist/helpers/reducer.mjs
var reducer = (accumulator, pair) => {
  const currentValue = accumulator[pair.name];
  if (currentValue === void 0) {
    accumulator[pair.name] = pair.value;
    return accumulator;
  }
  if (Array.isArray(currentValue)) {
    currentValue.push(pair.value);
    return accumulator;
  }
  accumulator[pair.name] = [currentValue, pair.value];
  return accumulator;
};

// node_modules/@readme/httpsnippet/dist/index.mjs
var import_url = __toESM(require_url(), 1);
var import_qs = __toESM(require_lib2(), 1);
var isHarEntry = (value) => typeof value === "object" && "log" in value && typeof value.log === "object" && "entries" in value.log && Array.isArray(value.log.entries);
var HTTPSnippet = class {
  constructor(input, opts = {}) {
    this.initCalled = false;
    this.entries = [];
    this.requests = [];
    this.options = {};
    this.options = {
      harIsAlreadyEncoded: false,
      ...opts
    };
    this.requests = [];
    if (isHarEntry(input)) this.entries = input.log.entries;
    else this.entries = [{ request: input }];
  }
  init() {
    this.initCalled = true;
    this.requests = this.entries.map(({ request }) => {
      const req = {
        bodySize: 0,
        headersSize: 0,
        headers: [],
        cookies: [],
        httpVersion: "HTTP/1.1",
        queryString: [],
        postData: { mimeType: request.postData?.mimeType || "application/octet-stream" },
        ...request
      };
      if (req.postData && !req.postData.mimeType) req.postData.mimeType = "application/octet-stream";
      return this.prepare(req, this.options);
    });
    return this;
  }
  prepare(harRequest, options) {
    const request = {
      ...harRequest,
      fullUrl: "",
      uriObj: {},
      queryObj: {},
      headersObj: {},
      cookiesObj: {},
      allHeaders: {}
    };
    if (request?.queryString.length) request.queryObj = request.queryString.reduce(reducer, /* @__PURE__ */ Object.create(null));
    if (request?.headers.length) {
      const http2VersionRegex = /^HTTP\/2/;
      request.headersObj = request.headers.reduce((accumulator, { name, value }) => {
        const headerName = http2VersionRegex.exec(request.httpVersion) ? name.toLocaleLowerCase() : name;
        return {
          ...accumulator,
          [headerName]: value
        };
      }, {});
    }
    if (request?.cookies.length) request.cookiesObj = request.cookies.reduceRight((accumulator, { name, value }) => ({
      ...accumulator,
      [name]: value
    }), {});
    const cookies = request.cookies?.map(({ name, value }) => {
      if (options.harIsAlreadyEncoded) return `${name}=${value}`;
      return `${encodeURIComponent(name)}=${encodeURIComponent(value)}`;
    });
    if (cookies?.length) request.allHeaders.cookie = cookies.join("; ");
    switch (request.postData.mimeType) {
      case "multipart/mixed":
      case "multipart/related":
      case "multipart/form-data":
      case "multipart/alternative":
        request.postData.text = "";
        request.postData.mimeType = "multipart/form-data";
        if (request.postData?.params) {
          const boundary = "---011000010111000001101001";
          const carriage = `${boundary}--`;
          const rn = "\r\n";
          /*! formdata-polyfill. MIT License. Jimmy Wärting <https://jimmy.warting.se/opensource> */
          const escapeStr = (str3) => str3.replace(/\n/g, "%0A").replace(/\r/g, "%0D").replace(/"/g, "%22");
          const normalizeLinefeeds = (value) => value.replace(/\r?\n|\r/g, "\r\n");
          const payload = [`--${boundary}`];
          request.postData?.params.forEach((param, i) => {
            const name = param.name;
            const value = param.value || "";
            const filename = param.fileName || null;
            const contentType = param.contentType || "application/octet-stream";
            if (filename) {
              payload.push(`Content-Disposition: form-data; name="${escapeStr(normalizeLinefeeds(name))}"; filename="${escapeStr(filename)}"`);
              payload.push(`Content-Type: ${contentType}`);
            } else payload.push(`Content-Disposition: form-data; name="${escapeStr(normalizeLinefeeds(name))}"`);
            payload.push("");
            payload.push(normalizeLinefeeds(value));
            if (i !== request.postData.params.length - 1) payload.push(`--${boundary}`);
          });
          payload.push(`--${carriage}`);
          request.postData.boundary = boundary;
          request.postData.text = payload.join(rn);
          const contentTypeHeader = getHeaderName(request.headersObj, "content-type") || "content-type";
          request.headersObj[contentTypeHeader] = `multipart/form-data; boundary=${boundary}`;
        }
        break;
      case "application/x-www-form-urlencoded":
        if (!request.postData.params) request.postData.text = "";
        else {
          request.postData.paramsObj = request.postData.params.reduce(reducer, /* @__PURE__ */ Object.create(null));
          request.postData.text = (0, import_qs.stringify)(request.postData.paramsObj);
        }
        break;
      case "text/json":
      case "text/x-json":
      case "application/json":
      case "application/x-json":
        request.postData.mimeType = "application/json";
        if (request.postData.text) try {
          request.postData.jsonObj = JSON.parse(request.postData.text);
        } catch {
          request.postData.mimeType = "text/plain";
        }
        break;
    }
    const allHeaders = {
      ...request.allHeaders,
      ...request.headersObj
    };
    const urlWithParsedQuery = (0, import_url.parse)(request.url, true, true);
    request.queryObj = {
      ...request.queryObj,
      ...urlWithParsedQuery.query
    };
    let search;
    if (options.harIsAlreadyEncoded) search = (0, import_qs.stringify)(request.queryObj, {
      encode: false,
      indices: false
    });
    else search = (0, import_qs.stringify)(request.queryObj, { indices: false });
    const uriObj = {
      ...urlWithParsedQuery,
      query: request.queryObj,
      search,
      path: search ? `${urlWithParsedQuery.pathname}?${search}` : urlWithParsedQuery.pathname
    };
    const url = (0, import_url.format)({
      ...urlWithParsedQuery,
      query: null,
      search: null
    });
    const fullUrl = (0, import_url.format)({
      ...urlWithParsedQuery,
      ...uriObj
    });
    return {
      ...request,
      allHeaders,
      fullUrl,
      url,
      uriObj
    };
  }
  convert(targetId, clientId, options) {
    if (!this.initCalled) this.init();
    if (!options && clientId) options = { clientId };
    const target = targets[targetId];
    if (!target) return [false];
    const { convert: convert2 } = target.clientsById[clientId || target.info.default];
    return this.requests.map((request) => convert2(request, options));
  }
  installation(targetId, clientId, options) {
    if (!this.initCalled) this.init();
    if (!options && clientId) options = { clientId };
    const target = targets[targetId];
    if (!target) return [false];
    const { info } = target.clientsById[clientId || target.info.default];
    return this.requests.map((request) => info?.installation ? info.installation(request, options) : false);
  }
};

// src/lib/snippets/comments.ts
var LINE_BREAKERS = /[\u0000-\u001f\u007f\u0085\u2028\u2029]/g;
function commentLine(language, text3) {
  let safe = text3.replace(LINE_BREAKERS, " ");
  if (language === "java") safe = safe.replace(/\\/g, "\\\\");
  if (language === "php") safe = safe.replace(/\?>/g, "? >");
  return language === "shell" || language === "python" ? `# ${safe}` : `// ${safe}`;
}

// src/lib/snippets/own/encode.ts
function shellQuote(s) {
  return `'${s.replaceAll("'", "'\\''")}'`;
}
function strictEncode(s) {
  return encodeURIComponent(s).replace(/[!'()*]/g, (c2) => `%${c2.charCodeAt(0).toString(16).toUpperCase()}`);
}
function formEncode(params2) {
  return params2.map((p) => `${strictEncode(p.name)}=${strictEncode(p.value)}`).join("&").replace(/%20/g, "+");
}
function buildURL(har) {
  if (!har.queryString.length) return har.url;
  return `${har.url}?${har.queryString.map((q) => `${strictEncode(q.name)}=${strictEncode(q.value)}`).join("&")}`;
}
function isMultipart(har) {
  return har.postData?.mimeType.toLowerCase().startsWith("multipart/form-data") ?? false;
}
function harBody(har) {
  if (har._tetiva?.binaryFile) return { kind: "file", name: har._tetiva.binaryFile };
  const params2 = har.postData?.params ?? [];
  if (params2.length) return isMultipart(har) ? { kind: "multipart", params: params2 } : { kind: "form", params: params2 };
  if (har.postData?.text) return { kind: "text", text: har.postData.text };
  return { kind: "none" };
}
function sentHeaders(har) {
  if (!isMultipart(har)) return har.headers;
  return har.headers.filter((h) => h.name.toLowerCase() !== "content-type");
}

// src/lib/snippets/vars.ts
function varRefs(s, from = 0, open = "{{") {
  const refs = [];
  let close = -1;
  let i = s.indexOf(open, from);
  while (i >= 0) {
    const nameStart = i + open.length;
    if (close < nameStart) close = s.indexOf("}", nameStart);
    if (close < 0) break;
    if (close > nameStart && s[close + 1] === "}") {
      refs.push({ start: i, end: close + 2, name: s.slice(nameStart, close) });
      i = s.indexOf(open, close + 2);
    } else {
      i = s.indexOf(open, i + 1);
    }
  }
  return refs;
}
function replaceVars(s, show, open = "{{") {
  let out = "";
  let last = 0;
  for (const r2 of varRefs(s, 0, open)) {
    out += s.slice(last, r2.start) + show(r2.name, s.slice(r2.start, r2.end));
    last = r2.end;
  }
  return last === 0 ? s : out + s.slice(last);
}

// src/lib/snippets/sentinel.ts
var PORT_AFTER_BASE = /^:(\d+|\{\{[^}]+\}\})(?=[/?#]|$)/;
var URL_PORT = /(:\/\/[^\s/?#'"`\\]*:)(\d+)(?!\d)/g;
var SAFE_NAME = /^[A-Za-z0-9_.-]+$/;
var PREFIX_RUN = /(?=zqv(z*))/g;
var FIRST_PORT = 59990;
var LAST_PORT = 59999;
function withSentinels(input) {
  const har = structuredClone(input);
  const seen = JSON.stringify(input);
  const lower = seen.toLowerCase();
  let run = -1;
  for (const m of lower.matchAll(PREFIX_RUN)) run = Math.max(run, m[1].length);
  const prefix = `zqv${"z".repeat(run + 1)}`;
  const tokenRe = new RegExp(`zqvz{${run + 1}}(\\d+)vqz`, "g");
  const names = [];
  const numbers = /* @__PURE__ */ new Map();
  const token = (name) => {
    let i = numbers.get(name);
    if (i === void 0) {
      i = names.push(name) - 1;
      numbers.set(name, i);
    }
    return `${prefix}${i}vqz`;
  };
  const encode = (s) => replaceVars(s, token);
  const refs = varRefs(har.url);
  for (const r2 of refs) token(r2.name);
  const hostAliases = [];
  let url = har.url;
  if (refs[0]?.start === 0) {
    const tok = token(refs[0].name);
    const host = `${tok}.invalid`;
    const rest = url.slice(refs[0].end);
    const slash = rest.startsWith("/") || PORT_AFTER_BASE.test(rest) ? "" : "/";
    url = `https://${host}${slash}${rest}`;
    if (slash) hostAliases.push([`https://${host}/`, tok]);
    hostAliases.push([`https://${host}`, tok], [host, tok]);
  }
  let nextPort = FIRST_PORT;
  const ports = /* @__PURE__ */ new Map();
  const portFor = (name) => {
    if (ports.has(name)) return ports.get(name);
    while (nextPort <= LAST_PORT && seen.includes(String(nextPort))) nextPort++;
    if (nextPort > LAST_PORT) return void 0;
    ports.set(name, nextPort);
    return nextPort++;
  };
  const authority = authorityRange(url);
  if (authority) {
    const [from, to] = authority;
    const protectedAuthority = replaceVars(url.slice(from, to), (name, whole) => {
      const port = portFor(name);
      return port === void 0 ? whole : `:${port}`;
    }, ":{{");
    url = url.slice(0, from) + protectedAuthority + url.slice(to);
  }
  const portTokens = new Map([...ports].map(([name, port]) => [String(port), token(name)]));
  har.url = encode(url);
  har.headers = har.headers.map((h) => ({ ...h, name: encode(h.name), value: encode(h.value) }));
  har.queryString = har.queryString.map((q) => ({ ...q, name: encode(q.name), value: encode(q.value) }));
  if (har.postData) {
    if (typeof har.postData.text === "string") har.postData.text = encode(har.postData.text);
    har.postData.params = har.postData.params?.map((p) => ({ ...p, name: encode(p.name), value: encode(p.value ?? "") }));
  }
  const warnings = [];
  const unsafe = /* @__PURE__ */ new Map();
  const shown = names.map((name) => showName(name, unsafe, warnings));
  const restore = (code) => {
    let out = code.replace(URL_PORT, (whole, head, port) => {
      const tok = portTokens.get(port);
      return tok ? head + tok : whole;
    });
    for (const [from, to] of hostAliases) out = out.replaceAll(from, to);
    return out.replace(tokenRe, (whole, i) => shown[Number(i)] ?? whole);
  };
  return { har, restore, warnings };
}
function restoreUnsafeNames(text3, seen = /* @__PURE__ */ new Map()) {
  const warnings = [];
  const out = replaceVars(text3, (name) => showName(name, seen, warnings));
  return { text: out, warnings };
}
function snippetNames() {
  const seen = /* @__PURE__ */ new Map();
  const warnings = [];
  const safe = (...parts) => parts.map((part) => {
    const r2 = restoreUnsafeNames(part, seen);
    warnings.push(...r2.warnings);
    return r2.text;
  }).join("");
  return { safe, warnings };
}
function showName(name, seen, warnings) {
  if (SAFE_NAME.test(name)) return `{{${name}}}`;
  let i = seen.get(name);
  if (i === void 0) {
    i = seen.size;
    seen.set(name, i);
    warnings.push(`Variable "${name}" contains unsafe characters and is shown as VAR_${i}`);
  }
  return `VAR_${i}`;
}
function authorityRange(url) {
  const scheme = url.indexOf("://");
  if (scheme < 0) return null;
  const from = scheme + 3;
  const refs = varRefs(url, from);
  let i = from;
  let next = 0;
  while (i < url.length) {
    if (refs[next]?.start === i) {
      i = refs[next++].end;
      continue;
    }
    if ("/?#".includes(url[i])) break;
    i++;
  }
  return [from, i];
}

// src/lib/snippets/grpc.ts
var PLAINTEXT = "Tetiva sends gRPC without TLS; drop -plaintext for TLS servers";
var NO_METHOD = "Choose a service and method; the command uses a placeholder";
function renderGrpcurl(g) {
  const names = snippetNames();
  const quote2 = (...parts) => shellQuote(names.safe(...parts));
  const args = ["-plaintext"];
  for (const key of Object.keys(g.metadata).sort()) {
    for (const value of g.metadata[key]) args.push(`-H ${quote2(key, ": ", value)}`);
  }
  if (g.message.trim()) args.push(`-d ${quote2(g.message)}`);
  const chosen = g.service.trim() !== "" && g.method.trim() !== "";
  args.push(quote2(g.target), chosen ? quote2(g.service, "/", g.method) : shellQuote("<service>/<method>"));
  const notes = chosen ? [] : [NO_METHOD];
  return { code: `grpcurl ${args.join(" \\\n  ")}`, warnings: [...names.warnings, ...notes, PLAINTEXT] };
}

// src/lib/snippets/types.ts
var Unsupported = class extends Error {
};

// src/lib/snippets/own/curl.ts
var GLOB_CHARS = /[[\]{}]/;
var BAD_FIELD_NAME = /^$|^[@<]|[=;"\p{Cc}]/u;
function renderCurl(har) {
  const lines = sentHeaders(har).map((h) => (
    // curl drops a header given as "Name:"; the semicolon form sends it with an empty value.
    `-H ${shellQuote(h.value ? `${h.name}: ${h.value}` : `${h.name};`)}`
  ));
  const body2 = harBody(har);
  if (body2.kind === "text") lines.push(`--data-raw ${shellQuote(body2.text)}`);
  else if (body2.kind === "form") lines.push(`--data-raw ${shellQuote(formEncode(body2.params))}`);
  else if (body2.kind === "file") lines.push(`--data-binary ${shellQuote(`@${body2.name}`)}`);
  else if (body2.kind === "multipart") lines.push(...body2.params.map(formPart));
  const url = buildURL(har);
  const globoff = GLOB_CHARS.test(url) ? " --globoff" : "";
  return [`curl${globoff}${methodFlag(har.method, body2.kind !== "none")} ${shellQuote(url)}`, ...lines].join(" \\\n  ");
}
function formPart(p) {
  if (BAD_FIELD_NAME.test(p.name)) throw new Unsupported(`Field name ${JSON.stringify(p.name)} is not supported by curl -F`);
  if (!p.fileName) return `--form-string ${shellQuote(`${p.name}=${p.value}`)}`;
  const file = formWord(p.fileName);
  return `-F ${shellQuote(`${p.name}=@${file};filename=${file}`)}`;
}
function formWord(s) {
  return `"${s.replace(/["\\]/g, "\\$&")}"`;
}
function methodFlag(method, hasBody) {
  if (method === "GET" && !hasBody) return "";
  if (method === "HEAD" && !hasBody) return " -I";
  return ` -X ${/^[A-Za-z0-9_-]+$/.test(method) ? method : shellQuote(method)}`;
}

// src/lib/snippets/own/fetch.ts
var str = (s) => JSON.stringify(s);
function renderFetch(har) {
  const lines = [`const url = ${str(buildURL(har))};`];
  const options = [`  method: ${str(har.method)},`];
  const headers2 = sentHeaders(har);
  if (headers2.length) options.push("  headers: {", ...headers2.map((h) => `    ${str(h.name)}: ${str(h.value)},`), "  },");
  const body2 = harBody(har);
  if (body2.kind === "text") options.push(`  body: ${str(body2.text)},`);
  else if (body2.kind === "form" || body2.kind === "multipart") {
    lines.push(`const body = new ${body2.kind === "form" ? "URLSearchParams" : "FormData"}();`);
    for (const p of body2.params) lines.push(`body.append(${str(p.name)}, ${str(p.value)});`);
    options.push("  body,");
  }
  return [
    ...lines,
    "const options = {",
    ...options,
    "};",
    "",
    "const response = await fetch(url, options);",
    "console.log(await response.text());"
  ].join("\n");
}

// src/lib/snippets/own/python.ts
var SHORT_ESCAPES = { "\\": "\\\\", '"': '\\"', "\n": "\\n", "\r": "\\r", "	": "\\t" };
function pyString(s) {
  const escaped = s.replace(/[\\"\u0000-\u001f\u007f-\u009f\u2028\u2029]/g, (c2) => SHORT_ESCAPES[c2] ?? `\\u${c2.charCodeAt(0).toString(16).padStart(4, "0")}`);
  return `"${escaped}"`;
}
function pyPair(name, value) {
  return `(${pyString(name)}, ${value})`;
}
function pyBlock(open, items2, close) {
  return [open, ...items2.map((i) => `    ${i},`), close].join("\n");
}
function renderPython(har) {
  const vars = [`url = ${pyString(har.url)}`];
  const args = [pyString(har.method), "url"];
  const assign = (name, value) => {
    vars.push(`${name} = ${value}`);
    args.push(`${name}=${name}`);
  };
  if (har.queryString.length) assign("params", pyBlock("[", har.queryString.map((q) => pyPair(q.name, pyString(q.value))), "]"));
  const headers2 = sentHeaders(har);
  if (headers2.length) assign("headers", pyBlock("{", headers2.map((h) => `${pyString(h.name)}: ${pyString(h.value)}`), "}"));
  const body2 = harBody(har);
  if (body2.kind === "text") assign("data", pyString(body2.text));
  else if (body2.kind === "file") assign("data", `open(${pyString(body2.name)}, "rb")`);
  else if (body2.kind === "form") assign("data", pyBlock("[", body2.params.map((p) => pyPair(p.name, pyString(p.value))), "]"));
  else if (body2.kind === "multipart") {
    const fields = body2.params.filter((p) => !p.fileName);
    const files = body2.params.filter((p) => p.fileName).map((p) => {
      const file = [pyString(p.fileName), `open(${pyString(p.fileName)}, "rb")`];
      if (p.contentType) file.push(pyString(p.contentType));
      return pyPair(p.name, `(${file.join(", ")})`);
    });
    if (!files.length) assign("files", pyBlock("[", fields.map((p) => pyPair(p.name, `(None, ${pyString(p.value)})`)), "]"));
    else {
      if (fields.length) assign("data", pyBlock("[", fields.map((p) => pyPair(p.name, pyString(p.value))), "]"));
      assign("files", pyBlock("[", files, "]"));
    }
  }
  return [
    "import requests",
    "",
    ...vars,
    "",
    `response = requests.request(${args.join(", ")})`,
    "print(response.text)"
  ].join("\n");
}

// src/lib/snippets/targets.ts
var SNIPPET_TARGET_META = [
  { key: "curl", label: "cURL", language: "shell", protocols: ["http", "graphql"], fileBodies: true },
  { key: "python-requests", label: "Python", language: "python", protocols: ["http", "graphql"], fileBodies: true },
  {
    key: "js-fetch",
    label: "JavaScript",
    language: "javascript",
    protocols: ["http", "graphql"],
    fileBodies: false,
    getBody: "refused"
  },
  { key: "go", label: "Go", language: "go", protocols: ["http", "graphql"], fileBodies: false },
  { key: "java-httpclient", label: "Java (HttpClient)", language: "java", protocols: ["http", "graphql"], fileBodies: false },
  {
    key: "java-okhttp",
    label: "Java (OkHttp)",
    language: "java",
    protocols: ["http", "graphql"],
    fileBodies: false,
    getBody: "dropped"
  },
  { key: "csharp-httpclient", label: "C#", language: "csharp", protocols: ["http", "graphql"], fileBodies: false },
  { key: "php-guzzle", label: "PHP", language: "php", protocols: ["http", "graphql"], fileBodies: false },
  { key: "grpcurl", label: "gRPCurl", language: "shell", protocols: ["grpc"], fileBodies: false },
  { key: "websocat", label: "websocat", language: "shell", protocols: ["websocket"], fileBodies: false },
  { key: "js-websocket", label: "JavaScript", language: "javascript", protocols: ["websocket"], fileBodies: false }
];

// src/lib/snippets/websocket.ts
var MULTILINE = "Each line becomes a separate message in websocat";
var NO_HEADERS = "Browsers cannot send custom WebSocket headers";
function headerPairs(headers2) {
  return Object.keys(headers2).sort().flatMap((k) => headers2[k].map((v) => [k, v]));
}
function renderWebsocat(w) {
  const names = snippetNames();
  const quote2 = (...parts) => shellQuote(names.safe(...parts));
  const notes = [];
  const flags = [];
  let pipe = "";
  const message = w.messages[0];
  if (message?.format === "binary") {
    pipe = `printf '%s' ${shellQuote(message.data)} | base64 -d | `;
    flags.push("--binary");
  } else if (message) {
    pipe = `printf '%s' ${quote2(message.data)} | `;
    if (message.data.includes("\n")) notes.push(MULTILINE);
  }
  if (w.subprotocols.length) flags.push(`--protocol=${shellQuote(w.subprotocols.map((s) => names.safe(s)).join(", "))}`);
  for (const [k, v] of headerPairs(w.headers)) flags.push(`-H=${quote2(k, ": ", v)}`);
  const code = `${pipe}websocat ${[...flags, quote2(w.url)].join(" \\\n  ")}`;
  return { code, warnings: [...names.warnings, ...notes] };
}
function renderJsWebSocket(w) {
  const names = snippetNames();
  const str3 = (s) => JSON.stringify(names.safe(s));
  const lines = [];
  const notes = [];
  const headers2 = headerPairs(w.headers);
  if (headers2.length) {
    notes.push(NO_HEADERS);
    lines.push(commentLine("javascript", "Browsers cannot send custom WebSocket headers; the request also has:"));
    for (const [k, v] of headers2) lines.push(commentLine("javascript", names.safe(k, ": ", v)));
  }
  const protocols = w.subprotocols.length ? `, [${w.subprotocols.map(str3).join(", ")}]` : "";
  lines.push(`const ws = new WebSocket(${str3(w.url)}${protocols});`);
  const message = w.messages[0];
  if (message) {
    const data = message.format === "binary" ? `Uint8Array.from(atob(${JSON.stringify(message.data)}), (c) => c.charCodeAt(0))` : str3(message.data);
    lines.push("", 'ws.addEventListener("open", () => {', `  ws.send(${data});`, "});");
  }
  lines.push("", 'ws.addEventListener("message", (e) => {', "  console.log(e.data);", "});");
  return { code: lines.join("\n"), warnings: [...names.warnings, ...notes] };
}

// src/lib/snippets/registry.ts
var IMPLS = {
  "curl": { kind: "own", render: renderCurl },
  "python-requests": { kind: "own", render: renderPython },
  "js-fetch": { kind: "own", render: renderFetch },
  "go": { kind: "library", target: "go", client: "native" },
  "java-httpclient": { kind: "library", target: "java", client: "nethttp" },
  "java-okhttp": { kind: "library", target: "java", client: "okhttp" },
  "csharp-httpclient": { kind: "library", target: "csharp", client: "httpclient" },
  "php-guzzle": { kind: "library", target: "php", client: "guzzle" },
  "grpcurl": { kind: "grpc", render: renderGrpcurl },
  "websocat": { kind: "ws", render: renderWebsocat },
  "js-websocket": { kind: "ws", render: renderJsWebSocket }
};
var SNIPPET_TARGETS = SNIPPET_TARGET_META.map((meta) => ({ ...meta, impl: IMPLS[meta.key] }));
function targetsFor(protocol) {
  return SNIPPET_TARGETS.filter((t) => t.protocols.includes(protocol));
}

// src/lib/snippets/generate.ts
var UNAVAILABLE = "Snippet unavailable for this language";
var AUTH_LABELS = /* @__PURE__ */ new Map([["digest", "Digest"], ["aws_sigv4", "AWS SigV4"]]);
var HEADER_TOKEN = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/;
var FORM = "application/x-www-form-urlencoded";
var NO_BODY_METHODS = /* @__PURE__ */ new Set(["GET", "HEAD"]);
var REENCODED = "Body re-encoded from its form fields and may differ from the raw text";
var HTTP_URL = /^https?:/i;
var NOT_HTTP = "Code is generated only for http and https URLs";
function generate(input, targetKey) {
  try {
    const target = SNIPPET_TARGETS.find((t) => t.key === targetKey);
    if (!target || !target.protocols.includes(input.protocol)) return { code: "", warnings: [UNAVAILABLE] };
    const copy = structuredClone(input);
    const impl = target.impl;
    if (impl.kind === "grpc") {
      const grpc = required(copy.grpc, "grpc");
      return withWarnings(keepTokenKeys(grpc.metadata), impl.render(grpc));
    }
    if (impl.kind === "ws") {
      const ws = required(copy.ws, "ws");
      return withWarnings(keepTokenKeys(ws.headers), impl.render(ws));
    }
    return generateHTTP(target, required(copy.har, "har"));
  } catch (e) {
    return { code: "", warnings: [`Snippet unavailable: ${e instanceof Error ? e.message : String(e)}`] };
  }
}
function generateHTTP(target, har) {
  dropUnset(har);
  const notes = [];
  const authNote = har._tetiva?.authNote;
  if (authNote) notes.push(`${AUTH_LABELS.get(authNote) ?? authNote} auth is computed at send time and is not included`);
  const warnings = checkBodilessMethod(target, har, notes);
  if (!target.fileBodies) notes.push(...dropFileBodies(har));
  const sentinels = withSentinels(har);
  if (!HTTP_URL.test(sentinels.har.url)) return { code: "", warnings: [NOT_HTTP] };
  warnings.push(...keepTokenHeaders(sentinels.har, har));
  const impl = target.impl;
  let code;
  if (impl.kind === "library") {
    warnings.push(...libraryForm(sentinels.har));
    code = convert(sentinels.har, impl.target, impl.client);
  } else if (impl.kind === "own") {
    try {
      code = impl.render(sentinels.har);
    } catch (e) {
      if (e instanceof Unsupported) return { code: "", warnings: [sentinels.restore(e.message), ...sentinels.warnings] };
      throw e;
    }
  } else throw new Error(`${impl.kind} target cannot print HTTP`);
  code = sentinels.restore(code);
  const comments = notes.map((n) => commentLine(target.language, n));
  return { code: prependComments(code, comments), warnings: [...warnings, ...sentinels.warnings] };
}
function dropUnset(har) {
  for (const key of Object.keys(har)) {
    if (har[key] === void 0 || har[key] === null) delete har[key];
  }
}
function keepTokenHeaders(encoded, original) {
  const warnings = [];
  encoded.headers = encoded.headers.filter((h, i) => {
    if (HEADER_TOKEN.test(h.name)) return true;
    warnings.push(badHeaderName(original.headers[i].name));
    return false;
  });
  return warnings;
}
function keepTokenKeys(headers2) {
  const warnings = [];
  for (const name of Object.keys(headers2)) {
    if (HEADER_TOKEN.test(replaceVars(name, () => "v"))) continue;
    warnings.push(badHeaderName(name));
    delete headers2[name];
  }
  return warnings;
}
function libraryForm(har) {
  const postData = har.postData;
  if (!postData || postData.mimeType.split(";")[0].trim().toLowerCase() !== FORM) return [];
  postData.mimeType = FORM;
  if (postData.params?.length || !postData.text) return [];
  const pairs = [...new URLSearchParams(postData.text)];
  postData.params = pairs.map(([name, value]) => ({ name, value }));
  const names = new Set(pairs.map(([name]) => name));
  return new URLSearchParams(pairs).toString() === postData.text && names.size === pairs.length ? [] : [REENCODED];
}
function badHeaderName(name) {
  return `Header ${JSON.stringify(name)} is not a valid HTTP header name and is left out`;
}
function withWarnings(warnings, result) {
  return { ...result, warnings: [...warnings, ...result.warnings] };
}
function prependComments(code, comments) {
  if (!comments.length) return code;
  const head = code.startsWith("<?php\n") ? "<?php\n" : "";
  return head + [...comments, code.slice(head.length)].join("\n");
}
function checkBodilessMethod(target, har, notes) {
  const method = har.method.toUpperCase();
  const hasBody = har._tetiva?.binaryFile || har.postData?.text || har.postData?.params?.length;
  if (!target.getBody || !NO_BODY_METHODS.has(method) || !hasBody) return [];
  const refused = target.getBody === "refused";
  const text3 = `${target.label} cannot send a body with ${method}; the body is ${refused ? "left out" : "not sent"}`;
  if (refused) {
    notes.push(text3);
    delete har.postData;
    delete har._tetiva?.binaryFile;
  }
  return [text3];
}
function dropFileBodies(har) {
  const notes = [];
  const binaryFile = har._tetiva?.binaryFile;
  if (binaryFile) {
    notes.push(`body: contents of file "${binaryFile}"`);
    delete har._tetiva.binaryFile;
    delete har.postData;
  }
  const params2 = har.postData?.params ?? [];
  if (params2.some((p) => p.fileName)) {
    for (const p of params2) if (p.fileName) notes.push(`attach file "${p.fileName}" as field "${p.name}"`);
    const fields = params2.filter((p) => !p.fileName);
    if (fields.length) har.postData.params = fields;
    else delete har.postData;
  }
  return notes;
}
function convert(har, target, client) {
  const out = new HTTPSnippet(har).convert(target, client)[0];
  if (typeof out !== "string") throw new Error(`${target}/${client} printed nothing`);
  return out;
}
function required(value, field) {
  if (value === void 0) throw new Error(`input has no ${field}`);
  return value;
}

// src/lib/snippets/snapshot/auth.ts
var RESERVED_HEADERS = /* @__PURE__ */ new Set(["host", "content-length", "authorization", "connection", "transfer-encoding"]);
var HMAC_ALGS = /* @__PURE__ */ new Set(["HS256", "HS384", "HS512"]);
var ASYMMETRIC_ALGS = /* @__PURE__ */ new Set(["RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512", "EdDSA", "none"]);
function locate(s, requestId) {
  const walk = (items2, ancestors) => {
    for (const item of items2 ?? []) {
      if (item?.kind === "request") {
        if (item.id === requestId) return { request: item, ancestors };
      } else if (item?.kind === "folder") {
        const found = walk(item.items, [...ancestors, item]);
        if (found) return found;
      }
    }
    return null;
  };
  const collection = s?.collection;
  return collection ? walk(collection.items, [collection]) : null;
}
function authOf(found) {
  const own = found.request.auth;
  if (own && own.type !== "inherit") return own;
  for (let i = found.ancestors.length - 1; i >= 0; i--) {
    const auth = found.ancestors[i].auth;
    if (auth && auth.type !== "none" && auth.type !== "inherit") return auth;
  }
  return null;
}
function effectiveAuth(s, requestId) {
  const found = locate(s, requestId);
  return found ? authOf(found) : null;
}
function applyAuth(auth, sub, headers2, overHTTP) {
  const out = { query: null, authNote: "", warnings: [] };
  if (!auth) return out;
  const f = isObject(auth.fields) ? auth.fields : {};
  const redacted = new Set(Array.isArray(auth.redacted) ? auth.redacted : []);
  const text3 = (key) => typeof f[key] === "string" ? sub(f[key]) : "";
  const empty = Object.keys(f).length === 0;
  const put = (addTo, param, prefix, token) => {
    if (addTo === "query") out.query = { key: param, value: token };
    else headers2.set("Authorization", [prefix === "" ? token : `${prefix} ${token}`]);
  };
  switch (auth.type) {
    case "none":
    case "inherit":
      return out;
    case "digest":
    case "aws_sigv4": {
      if (overHTTP) out.authNote = auth.type;
      else out.warnings.push(`${auth.type === "digest" ? "Digest" : "AWS Signature V4"} auth works over HTTP only and is left out of the snippet`);
      return out;
    }
    case "bearer": {
      if (empty) return out;
      put("header", "", "prefix" in f ? text3("prefix") : "Bearer", text3("token") || "<token>");
      return out;
    }
    case "basic": {
      if (empty) return out;
      const user = text3("username");
      const pass = text3("password");
      const refs = references([user, pass]);
      if (refs.length === 0 && !redacted.has("username") && !redacted.has("password")) {
        put("header", "", "Basic", base64(`${user}:${pass}`));
      } else {
        put("header", "", "Basic", "<credentials>");
        if (refs.length) out.warnings.push(`Basic auth is not encoded because these variables are not substituted: ${refs.join(", ")}`);
      }
      return out;
    }
    case "api_key": {
      if (empty) return out;
      const key = text3("key");
      const value = text3("value") || "<value>";
      if (key === "") return out;
      if ((text3("addTo") || text3("in")) === "query") {
        out.query = { key, value };
      } else if (RESERVED_HEADERS.has(key.toLowerCase())) {
        out.warnings.push(`API key header "${key}" is reserved and is left out of the snippet`);
      } else {
        headers2.set(key, [value]);
      }
      return out;
    }
    case "oauth2": {
      out.warnings.push("OAuth 2.0 token is not included");
      put(text3("addTo"), text3("queryParam") || "access_token", "headerPrefix" in f ? text3("headerPrefix") : "Bearer", "<token>");
      return out;
    }
    case "jwt": {
      if (empty) return out;
      const substituted = substituteDeep(f, sub);
      const [keyFields, otherFields] = jwtTokenFields(text3("alg") || "HS256");
      const refs = references([substituted.alg, ...keyFields.map((k) => substituted[k]), ...otherFields.map((k) => substituted[k])]);
      const keyRefs = references(keyFields.map((k) => substituted[k]));
      const token = keyRefs.length ? `<JWT signed with ${keyRefs.join(", ")}>` : "<JWT>";
      if (refs.length) out.warnings.push(`JWT is not signed because these variables are not substituted: ${refs.join(", ")}`);
      put(text3("addTo"), text3("queryParam") || "token", "headerPrefix" in f ? text3("headerPrefix") : "Bearer", token);
      return out;
    }
    default:
      out.warnings.push(`Auth type ${JSON.stringify(String(auth.type))} is not supported and is left out of the snippet`);
      return out;
  }
}
function jwtTokenFields(alg) {
  const other = ["expiresIn", "claims", "header"];
  if (HMAC_ALGS.has(alg)) return [["secret"], [...other, "secretBase64"]];
  if (ASYMMETRIC_ALGS.has(alg)) return [["privateKey"], other];
  return [["secret", "privateKey"], [...other, "secretBase64"]];
}
function references(values) {
  const refs = /* @__PURE__ */ new Set();
  const walk = (v) => {
    if (typeof v === "string") {
      for (const r2 of varRefs(v)) refs.add(v.slice(r2.start, r2.end));
    } else if (Array.isArray(v)) {
      v.forEach(walk);
    } else if (isObject(v)) {
      for (const k of Object.keys(v).sort()) walk(v[k]);
    }
  };
  values.forEach(walk);
  return [...refs];
}
function substituteDeep(v, sub) {
  if (typeof v === "string") return sub(v);
  if (Array.isArray(v)) return v.map((x) => substituteDeep(x, sub));
  if (isObject(v)) return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, substituteDeep(x, sub)]));
  return v;
}
function isObject(v) {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}
function base64(s) {
  let binary = "";
  for (const b of new TextEncoder().encode(s)) binary += String.fromCharCode(b);
  return btoa(binary);
}

// src/lib/snippets/snapshot/har.ts
var BINARY_WARNING = "Binary body is shown as a file reference";
var GRAPHQL_VARIABLES_WARNING = "GraphQL variables are not valid JSON; shown as written";
var AUTO_CONTENT_TYPE = {
  json: "application/json",
  xml: "application/xml",
  form: "application/x-www-form-urlencoded",
  binary: "application/octet-stream"
};
function harFromSnapshotRequest(req, env, auth) {
  return buildHar(req, env, auth)?.har ?? null;
}
function buildHar(req, env, auth) {
  const vars = publicVars(env);
  if (req?.protocol === "http" && req.http) return buildHTTP(req.http, vars, auth);
  if (req?.protocol === "graphql" && req.graphql) return buildGraphQL(req.graphql, vars, auth);
  return null;
}
function publicVars(env) {
  const vars = /* @__PURE__ */ new Map();
  const all = env?.variables;
  const rows = (Array.isArray(all) ? all : []).filter((v) => typeof v?.key === "string");
  const secret = new Set(rows.filter((v) => v.secret !== false).map((v) => v.key));
  for (const v of rows) {
    if (!secret.has(v.key) && typeof v.value === "string") vars.set(v.key, v.value);
  }
  return vars;
}
function substitute(text3, vars) {
  const s = str2(text3);
  if (vars.size === 0 || !s.includes("{{")) return s;
  return replaceVars(s, (name, match) => vars.get(name) ?? match);
}
function headerMap(rows, vars) {
  const out = /* @__PURE__ */ new Map();
  for (const h of Array.isArray(rows) ? rows : []) {
    if (!h?.enabled || !str2(h.key)) continue;
    const key = substitute(h.key, vars);
    const value = substitute(h.value, vars);
    const values = out.get(key);
    if (values) values.push(value);
    else out.set(key, [value]);
  }
  return out;
}
function setQueryParam(rawURL, key, value) {
  const spans = varSpans(rawURL);
  let base = rawURL;
  let fragment = "";
  const hash = indexOutside(base, "#", 0, spans);
  if (hash >= 0) [base, fragment] = [base.slice(0, hash), base.slice(hash)];
  let query = "";
  const q = indexOutside(base, "?", 0, spans);
  if (q >= 0) [base, query] = [base.slice(0, q), base.slice(q + 1)];
  const values = parseValues(query);
  values.set(key, [value]);
  const encoded = [...values.keys()].sort(compareCodePoints).flatMap((k) => values.get(k).map((v) => `${queryEscape(k)}=${queryEscape(v)}`)).join("&");
  return `${base}?${encoded}${fragment}`;
}
function buildHTTP(p, vars, auth) {
  const headers2 = headerMap(p.headers, vars);
  const applied = applyAuth(auth, (s) => substitute(s, vars), headers2, true);
  let url = substitute(p.url, vars);
  if (applied.query) url = setQueryParam(url, applied.query.key, applied.query.value);
  const b = p.body;
  const type = str2(b?.type);
  const auto = AUTO_CONTENT_TYPE[type];
  if (auto && !headerKey(headers2, "Content-Type")) headers2.set("Content-Type", [auto]);
  let body2 = { kind: "none" };
  switch (type) {
    case "none":
      break;
    case "form": {
      const fields = (Array.isArray(b.fields) ? b.fields : []).map((f) => ({ key: substitute(f?.key, vars), value: substitute(f?.value, vars), type: f?.type, enabled: f?.enabled === true })).filter((f) => f.enabled && f.key !== "");
      const multipart = fields.some((f) => f.type === "file");
      const params2 = fields.map((f) => f.type === "file" && f.value !== "" ? { name: f.key, value: "", fileName: f.value } : { name: f.key, value: f.value });
      if (params2.length) {
        body2 = multipart ? { kind: "multipart", mimeType: "multipart/form-data", params: params2 } : { kind: "urlencoded", mimeType: firstHeader(headers2, "Content-Type"), params: params2 };
      }
      break;
    }
    case "binary": {
      const file = substitute(b.fileName, vars);
      if (file) body2 = { kind: "binary", mimeType: firstHeader(headers2, "Content-Type"), file };
      break;
    }
    default: {
      let text3 = substitute(b?.raw, vars);
      if (type === "json") text3 = stripJSONC(text3);
      if (text3) body2 = { kind: "text", mimeType: firstHeader(headers2, "Content-Type"), text: text3 };
    }
  }
  return build(str2(p.method), url, headers2, body2, applied.authNote, applied.warnings);
}
function buildGraphQL(p, vars, auth) {
  const headers2 = headerMap(p.headers, vars);
  const applied = applyAuth(auth, (s) => substitute(s, vars), headers2, false);
  let url = substitute(p.url, vars);
  if (applied.query) url = setQueryParam(url, applied.query.key, applied.query.value);
  const [text3, bodyWarnings] = graphQLBodyLenient(substitute(p.query, vars), substitute(p.variables, vars), str2(p.operationName));
  let mimeType = firstHeader(headers2, "Content-Type");
  if (mimeType === "") {
    mimeType = "application/json";
    headers2.set("Content-Type", [mimeType]);
  }
  return build("POST", url, headers2, { kind: "text", mimeType, text: text3 }, "", [...applied.warnings, ...bodyWarnings]);
}
function build(method, rawURL, headers2, body2, authNote, warnings) {
  const [url, rawQuery] = splitURL(rawURL);
  const out = [...warnings];
  const harHeaders = buildHeaders(headers2, body2.kind === "multipart", out);
  let postData;
  let binaryFile = "";
  switch (body2.kind) {
    case "text":
      postData = { mimeType: body2.mimeType, text: body2.text, params: [] };
      break;
    case "urlencoded":
      postData = { mimeType: body2.mimeType || "application/x-www-form-urlencoded", text: "", params: body2.params.map((p) => ({ ...p })) };
      out.push(...repeatedKeyWarnings(body2.params));
      break;
    case "multipart":
      postData = {
        mimeType: body2.mimeType,
        text: "",
        params: body2.params.map((p) => p.fileName ? { name: p.name, value: "", fileName: baseName(p.fileName) } : { ...p })
      };
      break;
    case "binary":
      binaryFile = baseName(body2.file);
      postData = { mimeType: body2.mimeType, text: "", params: [] };
      out.push(BINARY_WARNING);
      break;
  }
  const har = {
    method,
    url,
    httpVersion: "HTTP/1.1",
    headers: harHeaders,
    queryString: parseQuery(rawQuery),
    cookies: [],
    ...postData ? { postData } : {},
    headersSize: -1,
    bodySize: -1
  };
  if (binaryFile || authNote) har._tetiva = { ...binaryFile ? { binaryFile } : {}, ...authNote ? { authNote } : {} };
  return { har, warnings: out };
}
function buildHeaders(headers2, dropContentType, warnings) {
  const keys = [...headers2.keys()].filter((k) => headers2.get(k).length > 0 && !(dropContentType && k.toLowerCase() === "content-type")).sort(compareCodePoints);
  return keys.map((name) => {
    const value = headers2.get(name).join(name.toLowerCase() === "cookie" ? "; " : ", ");
    if (!/^[\x00-\x7f]*$/.test(value)) warnings.push(`Header ${JSON.stringify(name)} has non-ASCII characters; some languages cannot send it`);
    return { name, value };
  });
}
function repeatedKeyWarnings(params2) {
  const seen = /* @__PURE__ */ new Map();
  const warnings = [];
  for (const p of params2) {
    const n = (seen.get(p.name) ?? 0) + 1;
    seen.set(p.name, n);
    if (n === 2) warnings.push(`Repeated form key ${JSON.stringify(p.name)}: some languages keep only the last value`);
  }
  return warnings;
}
function splitURL(raw) {
  const spans = varSpans(raw);
  let query = "";
  const hash = indexOutside(raw, "#", 0, spans);
  if (hash >= 0) raw = raw.slice(0, hash);
  const q = indexOutside(raw, "?", 0, spans);
  if (q >= 0) [raw, query] = [raw.slice(0, q), raw.slice(q + 1)];
  const scheme = indexOutside(raw, "://", 0, spans);
  if (scheme < 0) return [raw, query];
  const start = scheme + 3;
  let end = indexOutside(raw, "/", start, spans);
  if (end < 0) end = raw.length;
  for (let i = end - 1; i >= start; i--) {
    if (raw[i] === "@" && outside(i, spans)) return [raw.slice(0, start) + raw.slice(i + 1), query];
  }
  return [raw, query];
}
function parseQuery(q) {
  const out = [];
  for (const seg of splitOutside(q, "&")) {
    if (seg === "") continue;
    let [name, value] = [seg, ""];
    const eq = indexOutside(seg, "=", 0, varSpans(seg));
    if (eq >= 0) [name, value] = [seg.slice(0, eq), seg.slice(eq + 1)];
    out.push({ name: unescapeOutside(name, true), value: unescapeOutside(value, true) });
  }
  return out;
}
function parseValues(q) {
  const values = /* @__PURE__ */ new Map();
  for (const seg of splitOutside(q, "&")) {
    if (seg === "") continue;
    const spans = varSpans(seg);
    if (indexOutside(seg, ";", 0, spans) >= 0) continue;
    let [name, value] = [seg, ""];
    const eq = indexOutside(seg, "=", 0, spans);
    if (eq >= 0) [name, value] = [seg.slice(0, eq), seg.slice(eq + 1)];
    const k = unescapeOutside(name, false);
    const v = unescapeOutside(value, false);
    if (k === null || v === null) continue;
    const list = values.get(k);
    if (list) list.push(v);
    else values.set(k, [v]);
  }
  return values;
}
function unescapeOutside(s, keepBroken) {
  let out = "";
  let last = 0;
  for (const [start, end] of [...varSpans(s), [s.length, s.length]]) {
    const piece = s.slice(last, start);
    const decoded = queryUnescape(piece);
    if (decoded === null && !keepBroken) return null;
    out += (decoded ?? piece) + s.slice(start, end);
    last = end;
  }
  return out;
}
function queryUnescape(s) {
  if (!s.includes("%") && !s.includes("+")) return s;
  const bytes = [];
  const utf8 = new TextEncoder();
  for (let i = 0; i < s.length; i++) {
    const c2 = s[i];
    if (c2 === "%") {
      const hex = s.slice(i + 1, i + 3);
      if (!/^[0-9A-Fa-f]{2}$/.test(hex)) return null;
      bytes.push(parseInt(hex, 16));
      i += 2;
    } else if (c2 === "+") {
      bytes.push(32);
    } else {
      const cp = s.codePointAt(i);
      if (cp > 65535) i++;
      bytes.push(...utf8.encode(String.fromCodePoint(cp)));
    }
  }
  return new TextDecoder().decode(new Uint8Array(bytes));
}
function queryEscape(s) {
  let out = "";
  let last = 0;
  for (const [start, end] of [...varSpans(s), [s.length, s.length]]) {
    for (const b of new TextEncoder().encode(s.slice(last, start))) {
      const c2 = String.fromCharCode(b);
      if (/[A-Za-z0-9\-_.~]/.test(c2)) out += c2;
      else if (c2 === " ") out += "+";
      else out += `%${b.toString(16).toUpperCase().padStart(2, "0")}`;
    }
    out += s.slice(start, end);
    last = end;
  }
  return out;
}
function baseName(path) {
  const spans = varSpans(path);
  let end = path.length;
  while (end > 0 && (path[end - 1] === "/" || path[end - 1] === "\\")) end--;
  const trimmed = path.slice(0, end);
  for (let i = trimmed.length - 1; i >= 0; i--) {
    if ((trimmed[i] === "/" || trimmed[i] === "\\") && outside(i, spans)) return trimmed.slice(i + 1);
  }
  return trimmed;
}
function graphQLBodyLenient(query, variables, operationName) {
  const trimmed = variables.trim();
  if (trimmed === "" || trimmed === "{}") return [graphQLBody(query, "", operationName), []];
  const compact = compactJSON(variables);
  if (compact !== null) return [graphQLBody(query, compact, operationName), []];
  return [graphQLBody(query, trimmed, operationName), [GRAPHQL_VARIABLES_WARNING]];
}
function graphQLBody(query, variables, operationName) {
  let body2 = `{"query":${jsonString(query)}`;
  if (variables) body2 += `,"variables":${variables}`;
  if (operationName) body2 += `,"operationName":${jsonString(operationName)}`;
  return `${body2}}`;
}
function jsonString(s) {
  return JSON.stringify(s).replace(/\u2028/g, "\\u2028").replace(/\u2029/g, "\\u2029");
}
function compactJSON(s) {
  try {
    JSON.parse(s);
  } catch {
    return null;
  }
  let out = "";
  let inString = false;
  for (let i = 0; i < s.length; i++) {
    const c2 = s[i];
    if (inString) {
      out += c2;
      if (c2 === "\\") out += s[++i];
      else if (c2 === '"') inString = false;
    } else if (c2 === '"') {
      inString = true;
      out += c2;
    } else if (c2 !== " " && c2 !== "	" && c2 !== "\n" && c2 !== "\r") {
      out += c2;
    }
  }
  return out;
}
function stripJSONC(src) {
  let out = "";
  const n = src.length;
  let i = 0;
  while (i < n) {
    const c2 = src[i];
    if (c2 === '"') {
      out += c2;
      i++;
      while (i < n) {
        const ch = src[i++];
        out += ch;
        if (ch === "\\" && i < n) {
          out += src[i++];
          continue;
        }
        if (ch === '"') break;
      }
      continue;
    }
    if (c2 === "/" && src[i + 1] === "/") {
      i += 2;
      while (i < n && src[i] !== "\n") i++;
      continue;
    }
    if (c2 === "/" && src[i + 1] === "*") {
      i += 2;
      while (i < n) {
        if (src[i] === "\n") {
          out += "\n";
          i++;
          continue;
        }
        if (src[i] === "*" && src[i + 1] === "/") {
          i += 2;
          break;
        }
        i++;
      }
      continue;
    }
    out += c2;
    i++;
  }
  return out;
}
function headerKey(headers2, name) {
  const lower = name.toLowerCase();
  for (const k of headers2.keys()) if (k.toLowerCase() === lower) return k;
  return void 0;
}
function firstHeader(headers2, name) {
  const key = headerKey(headers2, name);
  return key === void 0 ? "" : headers2.get(key)?.[0] ?? "";
}
function varSpans(s) {
  return varRefs(s).map((r2) => [r2.start, r2.end]);
}
function outside(i, spans) {
  let lo = 0;
  let hi = spans.length;
  while (lo < hi) {
    const mid = lo + hi >> 1;
    if (spans[mid][1] <= i) lo = mid + 1;
    else hi = mid;
  }
  return lo === spans.length || i < spans[lo][0];
}
function indexOutside(s, sub, from, spans) {
  for (let i = s.indexOf(sub, from); i >= 0; i = s.indexOf(sub, i + 1)) {
    if (outside(i, spans)) return i;
  }
  return -1;
}
function splitOutside(s, sep) {
  const spans = varSpans(s);
  const parts = [];
  let last = 0;
  for (let i = 0; i < s.length; i++) {
    if (s[i] === sep && outside(i, spans)) {
      parts.push(s.slice(last, i));
      last = i + 1;
    }
  }
  parts.push(s.slice(last));
  return parts;
}
function compareCodePoints(a, b) {
  const x = [...a];
  const y = [...b];
  for (let i = 0; i < Math.min(x.length, y.length); i++) {
    const d = x[i].codePointAt(0) - y[i].codePointAt(0);
    if (d !== 0) return d;
  }
  return x.length - y.length;
}
function str2(v) {
  return typeof v === "string" ? v : "";
}

// src/lib/snippets/snapshot/input.ts
function snippetInputFromSnapshot(s, requestId, env) {
  const found = locate(s, requestId);
  if (!found) return null;
  const req = found.request;
  switch (req.protocol) {
    case "http":
    case "graphql": {
      const built = buildHar(req, env, authOf(found));
      return built && { protocol: req.protocol, har: built.har, warnings: built.warnings };
    }
    case "grpc": {
      const g = req.grpc;
      if (!g) return null;
      const vars = publicVars(env);
      return {
        protocol: "grpc",
        grpc: {
          target: substitute(g.target, vars),
          service: text(g.service),
          method: text(g.method),
          message: substitute(g.message, vars),
          metadata: Object.fromEntries(headerMap(g.metadata, vars))
        },
        warnings: []
      };
    }
    case "websocket": {
      const w = req.websocket;
      if (!w) return null;
      const vars = publicVars(env);
      const headers2 = headerMap(w.headers, vars);
      const applied = applyAuth(authOf(found), (t) => substitute(t, vars), headers2, false);
      let url = substitute(w.url, vars);
      if (applied.query) url = setQueryParam(url, applied.query.key, applied.query.value);
      const messages = (Array.isArray(w.messages) ? w.messages : []).map((m) => ({
        name: text(m?.name),
        format: m?.format,
        data: m?.format === "binary" ? text(m.data) : substitute(m?.data, vars)
      }));
      return {
        protocol: "websocket",
        ws: {
          url,
          headers: Object.fromEntries(headers2),
          subprotocols: (Array.isArray(w.subprotocols) ? w.subprotocols : []).filter((p) => typeof p === "string"),
          messages
        },
        warnings: applied.warnings
      };
    }
    default:
      return null;
  }
}
function text(v) {
  return typeof v === "string" ? v : "";
}

// src/lib/snippets/snapshot/postman.ts
var SCHEMA = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json";
var POSTMAN_GRANTS = {
  client_credentials: "client_credentials",
  password: "password_credentials",
  authorization_code: "authorization_code",
  device_code: "device_code"
};
function snapshotToPostman(s) {
  const warnings = [];
  const c2 = s.collection;
  const pm = {
    info: { name: text2(c2?.name), ...description(c2?.description), schema: SCHEMA },
    item: items(c2?.items, warnings),
    ...folderAuth(c2?.auth, `Collection ${JSON.stringify(text2(c2?.name))}`, warnings),
    ...events(c2?.scripts)
  };
  const variable = publicVariables(s.environment);
  if (variable.length) pm.variable = variable;
  return { json: `${JSON.stringify(pm, null, "	")}
`, warnings };
}
function items(list, warnings) {
  const out = [];
  for (const item of Array.isArray(list) ? list : []) {
    if (item?.kind === "folder") {
      out.push({
        name: text2(item.name),
        ...description(item.description),
        item: items(item.items, warnings),
        ...folderAuth(item.auth, `Folder ${JSON.stringify(text2(item.name))}`, warnings),
        ...events(item.scripts)
      });
    } else if (item?.kind === "request") {
      const converted = requestItem(item, warnings);
      if (converted) out.push(converted);
    }
  }
  return out;
}
function requestItem(r2, warnings) {
  const label = `Request ${JSON.stringify(text2(r2.name))}`;
  let request;
  if (r2.protocol === "http" && r2.http) {
    request = {
      method: text2(r2.http.method),
      header: headers(r2.http.headers),
      ...body(r2.http.body, label, warnings),
      url: { raw: text2(r2.http.url) }
    };
  } else if (r2.protocol === "graphql" && r2.graphql) {
    const variables = text2(r2.graphql.variables);
    request = {
      method: "POST",
      header: headers(r2.graphql.headers),
      body: { mode: "graphql", graphql: { query: text2(r2.graphql.query), ...variables ? { variables } : {} } },
      url: { raw: text2(r2.graphql.url) }
    };
  } else {
    const kind = r2.protocol === "grpc" ? "gRPC" : r2.protocol === "websocket" ? "WebSocket" : JSON.stringify(String(r2.protocol));
    warnings.push(`${label} was skipped: Postman collections cannot hold ${kind} requests`);
    return null;
  }
  const original = { ...request };
  const auth = requestAuth(r2.auth, label, warnings);
  return {
    name: text2(r2.name),
    request: { ...request, ...auth, ...description(r2.description) },
    response: (Array.isArray(r2.examples) ? r2.examples : []).map((e) => response(e, original)),
    ...events(r2.scripts)
  };
}
function headers(rows) {
  return (Array.isArray(rows) ? rows : []).map((h) => ({
    key: text2(h?.key),
    value: text2(h?.value),
    ...h?.enabled === false ? { disabled: true } : {}
  }));
}
function body(b, label, warnings) {
  const attach = (name) => warnings.push(`${label}: attach file ${JSON.stringify(name)} in Postman`);
  switch (b?.type) {
    case "json":
    case "xml":
    case "raw": {
      const raw = text2(b.raw);
      if (!raw) return {};
      return { body: { mode: "raw", raw, ...b.type === "raw" ? {} : { options: { raw: { language: b.type } } } } };
    }
    case "form": {
      const fields = Array.isArray(b.fields) ? b.fields : [];
      if (fields.length === 0) return {};
      const disabled = (enabled) => enabled === false ? { disabled: true } : {};
      if (!fields.some((f) => f?.enabled && f.type === "file" && text2(f.key))) {
        return {
          body: {
            mode: "urlencoded",
            urlencoded: fields.map((f) => ({ key: text2(f?.key), value: f?.type === "file" ? "" : text2(f?.value), ...disabled(f?.enabled) }))
          }
        };
      }
      return {
        body: {
          mode: "formdata",
          formdata: fields.map((f) => {
            if (f?.type !== "file") return { key: text2(f?.key), value: text2(f?.value), type: "text", ...disabled(f?.enabled) };
            const name = text2(f.value);
            if (name && f.enabled) attach(name);
            return { key: text2(f.key), type: "file", ...disabled(f.enabled), ...name ? { description: name } : {} };
          })
        }
      };
    }
    case "binary": {
      const name = text2(b.fileName);
      if (name) attach(name);
      return { body: { mode: "file", file: {} } };
    }
    default:
      return {};
  }
}
function response(e, originalRequest) {
  const contentType = text2(e?.contentType).toLowerCase();
  const language = ["json", "xml", "html"].find((l) => contentType.includes(l)) ?? "text";
  return {
    name: text2(e?.name),
    originalRequest,
    status: text2(e?.statusText),
    code: typeof e?.status === "number" ? e.status : 0,
    _postman_previewlanguage: language,
    header: headers(e?.headers),
    body: text2(e?.body)
  };
}
function requestAuth(a, label, warnings) {
  if (!a || a.type === "inherit") return {};
  if (a.type === "none") return { auth: { type: "noauth" } };
  const block = authBlock(a, label, warnings);
  return block ? { auth: block } : {};
}
function folderAuth(a, label, warnings) {
  if (!a || a.type === "none" || a.type === "inherit") return {};
  const block = authBlock(a, label, warnings);
  return block ? { auth: block } : {};
}
function authBlock(a, label, warnings) {
  const f = typeof a.fields === "object" && a.fields !== null && !Array.isArray(a.fields) ? a.fields : {};
  const str3 = (key) => text2(f[key]);
  const kv = (key, value) => ({ key, value });
  switch (a.type) {
    case "bearer":
      return { type: "bearer", bearer: [kv("token", str3("token"))] };
    case "basic":
      return { type: "basic", basic: [kv("username", str3("username")), kv("password", str3("password"))] };
    case "api_key":
      return { type: "apikey", apikey: [kv("key", str3("key")), kv("value", str3("value")), kv("in", str3("addTo") || str3("in"))] };
    case "digest":
      return { type: "digest", digest: [kv("username", str3("username")), kv("password", str3("password"))] };
    case "aws_sigv4": {
      const kvs = [kv("accessKey", str3("accessKeyId")), kv("secretKey", str3("secretAccessKey")), kv("region", str3("region")), kv("service", str3("service"))];
      if (str3("sessionToken")) kvs.push(kv("sessionToken", str3("sessionToken")));
      return { type: "awsv4", awsv4: kvs };
    }
    case "oauth2": {
      const kvs = [];
      const add = (key, value) => {
        if (value) kvs.push(kv(key, value));
      };
      add("grant_type", POSTMAN_GRANTS[str3("grant")] ?? str3("grant"));
      add("accessTokenUrl", str3("tokenUrl"));
      add("authUrl", str3("authUrl"));
      add("clientId", str3("clientId"));
      add("clientSecret", str3("clientSecret"));
      add("scope", str3("scope"));
      add("audience", str3("audience"));
      add("username", str3("username"));
      add("password", str3("password"));
      add("headerPrefix", str3("headerPrefix"));
      const clientAuth = str3("clientAuth");
      if (clientAuth === "basic") add("client_authentication", "header");
      else if (clientAuth === "body") add("client_authentication", "body");
      else if (clientAuth === "none") add("tetivaClientAuth", "none");
      add("addTokenTo", addTokenTo(str3("addTo")));
      add("tetivaQueryParam", fieldText(f.queryParam));
      add("tetivaRedirectPort", fieldText(f.redirectPort));
      add("tetivaDeviceAuthUrl", fieldText(f.deviceAuthUrl));
      return { type: "oauth2", oauth2: kvs };
    }
    case "jwt": {
      const kvs = [];
      const add = (key, value) => {
        if (value) kvs.push(kv(key, value));
      };
      add("algorithm", str3("alg"));
      add("secret", str3("secret"));
      add("privateKey", str3("privateKey"));
      if ("secretBase64" in f) kvs.push({ key: "isSecretBase64Encoded", value: str3("secretBase64") === "true" });
      add("payload", objectText(f.claims));
      add("header", objectText(f.header));
      add("headerPrefix", str3("headerPrefix"));
      add("queryParamKey", str3("queryParam"));
      add("addTokenTo", addTokenTo(str3("addTo")));
      add("tetivaExpiresIn", fieldText(f.expiresIn));
      return { type: "jwt", jwt: kvs };
    }
    default:
      warnings.push(`${label}: auth type ${JSON.stringify(String(a.type))} is not supported and was left out`);
      return null;
  }
}
function events(scripts) {
  const list = [];
  const add = (listen, script) => {
    if (script) list.push({ listen, script: { type: "text/javascript", exec: script.split("\n") } });
  };
  add("prerequest", text2(scripts?.pre));
  add("test", text2(scripts?.post));
  return list.length ? { event: list } : {};
}
function publicVariables(env) {
  const all = env?.variables;
  return (Array.isArray(all) ? all : []).filter((v) => v?.secret === false && typeof v.key === "string").map((v) => ({ key: v.key, value: text2(v.value) }));
}
function description(s) {
  const d = text2(s);
  return d ? { description: d } : {};
}
function addTokenTo(v) {
  return v === "header" ? "header" : v === "query" ? "queryParams" : "";
}
function fieldText(v) {
  return typeof v === "string" ? v : typeof v === "number" ? String(v) : "";
}
function objectText(v) {
  if (typeof v !== "object" || v === null || Array.isArray(v) || Object.keys(v).length === 0) return "";
  return JSON.stringify(v);
}
function text2(v) {
  return typeof v === "string" ? v : "";
}
export {
  SNIPPET_TARGETS,
  effectiveAuth,
  generate,
  harFromSnapshotRequest,
  snapshotToPostman,
  snippetInputFromSnapshot,
  targetsFor
};
