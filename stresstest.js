const http = require('k6/http');
const { check, sleep } = require('k6');
import { b64encode } from 'k6/encoding';

export let options = {
  vus: 5,  // Number of virtual users
  duration: '100s',  // Test duration
};

//ec2-44-203-148-222.compute-1.amazonaws.com

export default function () {
  const results = {
    go: { totalTime: 0, count: 0 },
    cpp: { totalTime: 0, count: 0 },
    python: { totalTime: 0, count: 0 },
    js: { totalTime: 0, count: 0 },
  };

  // Test for Go code
  let startTime = new Date();
  let goRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "go",
    code: b64encode("package main; import \"fmt\"; func main() { fmt.Println(\"Hello\") }")
  }), { headers: { "Content-Type": "application/json" } });
  let goDuration = new Date() - startTime;
  results.go.totalTime += goDuration;
  results.go.count++;

  check(goRes, {
    "Go status is 200": (r) => r.status === 200,
    "Go execution is fast": (r) => r.timings.duration < 2000,
    "Go response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  // Test for C++ code
  startTime = new Date();
  let cppRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "cpp",
    code: b64encode("#include <iostream>\nint main() { std::cout << \"Hello from C++\" << std::endl; return 0; }")
  }), { headers: { "Content-Type": "application/json" } });
  let cppDuration = new Date() - startTime;
  results.cpp.totalTime += cppDuration;
  results.cpp.count++;

  check(cppRes, {
    "C++ status is 200": (r) => r.status === 200,
    "C++ execution is fast": (r) => r.timings.duration < 2000,
    "C++ response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  // Test for Python code
  startTime = new Date();
  let pythonRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "python",
    code: b64encode("print('Hello from Python')")
  }), { headers: { "Content-Type": "application/json" } });
  let pythonDuration = new Date() - startTime;
  results.python.totalTime += pythonDuration;
  results.python.count++;

  check(pythonRes, {
    "Python status is 200": (r) => r.status === 200,
    "Python execution is fast": (r) => r.timings.duration < 2000,
    "Python response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  // Test for Node.js code
  startTime = new Date();
  let nodeRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "js",
    code: b64encode("console.log('Hello from Node.js')")
  }), { headers: { "Content-Type": "application/json" } });
  let nodeDuration = new Date() - startTime;
  results.js.totalTime += nodeDuration;
  results.js.count++;

  check(nodeRes, {
    "Node.js status is 200": (r) => r.status === 200,
    "Node.js execution is fast": (r) => r.timings.duration < 2000,
    "Node.js response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  sleep(1);

  // Log average execution times
  for (const lang in results) {
    if (results[lang].count > 0) {
      const avgTime = results[lang].totalTime / results[lang].count;
      console.log(`Average execution time for ${lang}: ${avgTime.toFixed(2)} ms`);
    }
  }
}