import requests
import threading
import time
import json

URL = "http://localhost:8080/execute"

# curl -i localhost:8080/run -X POST --data '' -H 'Content-Type: application/json'
PAYLOAD = {"code":"package main\r\n\r\nimport \"fmt\"\r\n\r\nfunc main() {\r\n    fmt.Println(\"Hello, Go!\")\r\n}","id":"123","variant":"go","language":"go"}
HEADERS = {"Content-Type": "application/json"}
NUM_THREADS = 1  # Adjust as needed
NUM_REQUESTS_PER_THREAD = 100  # Adjust as needed

log_lock = threading.Lock()
request_count = 0

def send_request(thread_id):
    global request_count
    for _ in range(NUM_REQUESTS_PER_THREAD):
        start_time = time.time()
        try:
            response = requests.post(URL, data=json.dumps(PAYLOAD), headers=HEADERS, timeout=5)
            duration = time.time() - start_time
            with log_lock:
                request_count += 1
                print(f"Thread {thread_id}: Sent request {request_count},ResponeBody:{response.json()}, Status: {response.status_code}, Time: {duration:.4f}s")
        except requests.exceptions.RequestException as e:
            with log_lock:
                print(f"Thread {thread_id}: Request failed - {e}")

def stress_test():
    threads = []
    start_time = time.time()
    for i in range(NUM_THREADS):
        thread = threading.Thread(target=send_request, args=(i,))
        threads.append(thread)
        thread.start()
    
    for thread in threads:
        thread.join()
    
    total_time = time.time() - start_time
    print(f"Total requests sent: {request_count}")
    print(f"Total time taken: {total_time:.2f}s")
    print(f"Requests per second: {request_count / total_time:.2f}")

if __name__ == "__main__":
    stress_test()
