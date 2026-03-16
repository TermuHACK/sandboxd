package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

var (
	apiHostPass = os.Getenv("API_HOST_PASS")
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	sandboxes = make(map[string]*exec.Cmd)
	mu        sync.Mutex
	counter   int
)

func DetectEngine() string {
	cmd := exec.Command("bwrap", "--unshare-user", "--ro-bind", "/", "/", "echo", "ok")
	if err := cmd.Run(); err == nil {
		return "bwrap"
	}
	return "proot"
}

func main() {
	engine := DetectEngine()
	port := "8080"
	log.Printf("Sandboxd started on :%s using %s\n", port, engine)

	http.HandleFunc("/sandbox", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		mu.Lock()
		counter++
		id := fmt.Sprintf("sb-%d", counter) // Теперь fmt используется здесь
		mu.Unlock()

		var cmd *exec.Cmd
		if engine == "bwrap" {
			cmd = exec.Command("bwrap", "--unshare-all", "--cap-drop", "ALL", "--ro-bind", "/", "/", "--proc", "/proc", "--dev", "/dev", "/bin/sh")
		} else {
			cmd = exec.Command("proot", "-0", "-r", "/", "-b", "/dev", "-b", "/proc", "/bin/sh")
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		ptmx, err := pty.Start(cmd)
		if err != nil {
			log.Printf("[%s] Failed to start: %v", id, err)
			return
		}
		defer ptmx.Close()

		log.Printf("[%s] Session started", id)

		mu.Lock()
		sandboxes[id] = cmd
		mu.Unlock()

		// Копируем данные в обе стороны
		go io.Copy(ptmx, conn.UnderlyingConn())
		io.Copy(conn.UnderlyingConn(), ptmx)

		cmd.Process.Kill()
		cmd.Wait()

		mu.Lock()
		delete(sandboxes, id)
		mu.Unlock()
		log.Printf("[%s] Session ended", id)
	})

	http.HandleFunc("/host/shell", func(w http.ResponseWriter, r *http.Request) {
		if apiHostPass == "" || r.URL.Query().Get("token") != apiHostPass {
			http.Error(w, "Unauthorized", 401)
			return
		}
		conn, _ := upgrader.Upgrade(w, r, nil)
		cmd := exec.Command("/bin/sh")
		ptmx, _ := pty.Start(cmd)
		go io.Copy(ptmx, conn.UnderlyingConn())
		io.Copy(conn.UnderlyingConn(), ptmx)
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
