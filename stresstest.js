const http = require('k6/http');
const { check, sleep } = require('k6');

export let options = {
  vus: 5,  // Number of virtual users
  duration: '1000s',  // Test duration
};

//ec2-44-203-148-222.compute-1.amazonaws.com

export default function () {
  // Test for Go code
  let goRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "go",
    code: "package main; import \"fmt\"; func main() { fmt.Println(\"Hello\") }"
  }), { headers: { "Content-Type": "application/json" } });

  check(goRes, {
    "Go status is 200": (r) => r.status === 200,
    "Go execution is fast": (r) => r.timings.duration < 1000,
    "Go response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  // Test for C++ code
  let cppRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "cpp",
    code: "#include <iostream>\nint main() { std::cout << \"Hello\" << std::endl; return 0; }"
  }), { headers: { "Content-Type": "application/json" } });

  check(cppRes, {
    "C++ status is 200": (r) => r.status === 200,
    "C++ execution is fast": (r) => r.timings.duration < 1500,
    "C++ response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  // Test for Python code
  let pythonRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "python",
    code: "print('Hello from Python')"
  }), { headers: { "Content-Type": "application/json" } });

  check(pythonRes, {
    "Python status is 200": (r) => r.status === 200,
    "Python execution is fast": (r) => r.timings.duration < 1000,
    "Python response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  // Test for Node.js code
  let nodeRes = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "js",
    code: "console.log('Hello from Node.js')"
  }), { headers: { "Content-Type": "application/json" } });

  check(nodeRes, {
    "Node.js status is 200": (r) => r.status === 200,
    "Node.js execution is fast": (r) => r.timings.duration < 1000,
    "Node.js response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  sleep(1);
}