(function (chai, response, grpc) {
  'use strict';

  var Assertion = chai.Assertion;
  var util = chai.util;

  function parseJSON(text) {
    try {
      return { value: JSON.parse(text) };
    } catch (e) {
      return { error: e.message };
    }
  }

  Assertion.addMethod('status', function (expected) {
    var obj = this._obj;
    if (typeof expected === 'number') {
      this.assert(obj.code === expected,
        'expected response to have status code #{exp} but got #{act}',
        'expected response to not have status code #{act}',
        expected, obj.code);
      return;
    }
    this.assert(obj.status === expected,
      'expected response to have status reason #{exp} but got #{act}',
      'expected response to not have status reason #{act}',
      expected, obj.status);
  });

  Assertion.addMethod('header', function (name, value) {
    var actual = this._obj.headers.get(name);
    this.assert(actual !== undefined,
      'expected response to have header with key ' + util.objDisplay(name),
      'expected response to not have header with key ' + util.objDisplay(name));
    if (arguments.length < 2) return;
    this.assert(actual === value,
      'expected response header ' + util.objDisplay(name) + ' to be #{exp} but got #{act}',
      'expected response header ' + util.objDisplay(name) + ' to not be #{act}',
      value, actual);
  });

  Assertion.addMethod('body', function (expected) {
    var text = this._obj.text();
    if (arguments.length === 0) {
      this.assert(text !== '',
        'expected response to have content in body',
        'expected response to not have content in body');
      return;
    }
    if (expected instanceof RegExp) {
      this.assert(expected.test(text),
        'expected response body #{act} to match #{exp}',
        'expected response body #{act} to not match #{exp}',
        expected, text);
      return;
    }
    if (typeof expected === 'string') {
      this.assert(text === expected,
        'expected response body to equal #{exp} but got #{act}',
        'expected response body to not equal #{act}',
        expected, text);
      return;
    }
    var body = parseJSON(text);
    this.assert(util.eql(body.value, expected),
      'expected response body json to equal #{exp} but got #{act}',
      'expected response body json to not equal #{exp}',
      expected, body.value);
  });

  Assertion.addMethod('jsonBody', function (path, value) {
    var body = parseJSON(this._obj.text());
    if (body.error !== undefined || arguments.length === 0) {
      this.assert(body.error === undefined,
        'expected response body to be a valid json but got error #{act}',
        'expected response body to not be a valid json',
        undefined, body.error);
      return;
    }
    if (typeof path !== 'string') {
      this.assert(util.eql(body.value, path),
        'expected response body json to deep equal #{exp} but got #{act}',
        'expected response body json to not deep equal #{exp}',
        path, body.value);
      return;
    }
    var info = util.getPathInfo(body.value, path);
    if (arguments.length === 1) {
      this.assert(info.exists,
        'expected response body json to have path #{exp}',
        'expected response body json to not have path #{exp}',
        path);
      return;
    }
    this.assert(info.exists && util.eql(info.value, value),
      'expected response body json at ' + util.objDisplay(path) + ' to deep equal #{exp} but got #{act}',
      'expected response body json at ' + util.objDisplay(path) + ' to not deep equal #{exp}',
      value, info.value);
  });

  Assertion.addProperty('json', function () {
    var body = parseJSON(this._obj.text());
    this.assert(body.error === undefined,
      'expected response body to be a valid json but got error #{act}',
      'expected response body to not be a valid json',
      undefined, body.error);
  });

  var codeClasses = {
    info: [100, 199, '1XX'],
    success: [200, 299, '2XX'],
    redirection: [300, 399, '3XX'],
    clientError: [400, 499, '4XX'],
    serverError: [500, 599, '5XX'],
    error: [400, 599, '4XX or 5XX'],
    ok: [200, 200, '200'],
    accepted: [202, 202, '202'],
    badRequest: [400, 400, '400'],
    unauthorized: [401, 401, '401'],
    forbidden: [403, 403, '403'],
    notFound: [404, 404, '404'],
    rateLimited: [429, 429, '429']
  };
  if (grpc) {
    codeClasses.ok = codeClasses.success = [0, 0, '0 (OK)'];
    codeClasses.error = [1, 16, 'a gRPC error'];
  }

  function assertCodeClass(assertion, range) {
    var code = assertion._obj.code;
    assertion.assert(code >= range[0] && code <= range[1],
      'expected response code to be ' + range[2] + ' but found ' + code,
      'expected response code to not be ' + range[2] + ' but found ' + code);
  }

  Object.keys(codeClasses).forEach(function (name) {
    var range = codeClasses[name];
    if (name === 'ok') {
      Assertion.overwriteProperty('ok', function (_super) {
        return function () {
          if (response === undefined || this._obj !== response) return _super.call(this);
          assertCodeClass(this, range);
        };
      });
      return;
    }
    Assertion.addProperty(name, function () {
      assertCodeClass(this, range);
    });
  });
});
