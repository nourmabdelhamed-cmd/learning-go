package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
)

func main() {
	lake := flag.String("lake", "lake", "Parquet lake directory")
	flag.Parse()

	cmd := exec.Command("uv", "run", "python", "-m", "training_loader.query_demo", "--lake", *lake)
	cmd.Env = append(os.Environ(), "PYTHONPATH=python")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
