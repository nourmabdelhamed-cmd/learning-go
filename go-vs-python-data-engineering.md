# Go vs Python for Data Engineering Projects

This guide introduces Go from the point of view of a Python data engineer. It uses small pipeline examples to show where Go is useful, where Python is stronger, and how the tradeoffs appear in real code.

## What Is Go?

Go, often called Golang, is a compiled, statically typed language designed for simple syntax, fast builds, network services, command-line tools, and concurrent programs.

In data engineering, Go is a good fit for:

- ingestion services
- API collectors
- stream processors
- file validators
- metadata services
- data platform CLIs
- small workers that need predictable deployment

Python is usually stronger for:

- notebooks and exploration
- pandas, NumPy, and PySpark workflows
- ML feature engineering
- quick scripts
- large data ecosystem integrations

The practical rule is simple: use Python when the work is analytical and library-heavy; consider Go when the work is operational, concurrent, service-oriented, or needs a small deployable binary.

## Quick Comparison

| Area | Go | Python |
| --- | --- | --- |
| Typing | Static types catch many mistakes before runtime | Dynamic typing is flexible but mistakes appear at runtime |
| Speed | Fast compiled programs, good memory control | Fast enough for many scripts, but slower for CPU-heavy local code |
| Deployment | Single binary is easy to ship | Usually needs Python, dependencies, and environment management |
| Data ecosystem | Smaller data science ecosystem | Excellent data ecosystem: pandas, PySpark, Airflow, dbt integrations, ML tools |
| Concurrency | Goroutines and channels are built in | Async, threads, and processes exist, but have more tradeoffs |
| Learning curve | Simple syntax, stricter structure | Very beginner-friendly and concise |
| Best use | Reliable services, CLIs, ingestion, APIs, workers | Analysis, orchestration, Spark jobs, notebooks, ML pipelines |

## Example 1: Validating Incoming Events

Imagine a pipeline receives JSON events:

```json
{"user_id":"u-123","event_type":"click","timestamp":"2026-06-17T10:00:00Z"}
```

### Python Version

```python
import json

def parse_event(line: str) -> dict:
    event = json.loads(line)

    if not event.get("user_id"):
        raise ValueError("missing user_id")

    return event
```

Python pros in this example:

- Very short and easy to read.
- Good for quick ingestion prototypes.
- Easy to extend with pandas, PySpark, or validation libraries.

Python cons in this example:

- The event shape is not obvious unless you read the validation code.
- Missing fields or wrong types are discovered at runtime.
- A large project can drift into many loosely defined dictionaries.

### Go Version

```go
package main

import (
	"encoding/json"
	"errors"
)

type Event struct {
	UserID    string `json:"user_id"`
	EventType string `json:"event_type"`
	Timestamp string `json:"timestamp"`
}

func parseEvent(line []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(line, &event); err != nil {
		return Event{}, err
	}
	if event.UserID == "" {
		return Event{}, errors.New("missing user_id")
	}
	return event, nil
}
```

Go pros in this example:

- The event contract is visible in the `Event` struct.
- Field names and types are checked by the compiler in the rest of the program.
- This style works well for long-lived ingestion services.

Go cons in this example:

- More code is needed for the same small task.
- String timestamps still need explicit parsing if strict time validation is required.
- It is less convenient for ad hoc exploration than a Python notebook.

## Example 2: Processing Many Files

Data engineering often means processing many input files from object storage, local disk, or a landing zone.

### Python Version

```python
from pathlib import Path

def count_lines(path: Path) -> int:
    with path.open() as file:
        return sum(1 for _ in file)

for path in Path("landing").glob("*.jsonl"):
    print(path.name, count_lines(path))
```

Python pros:

- Excellent for small scripts and one-off checks.
- Very readable for file system work.
- Easy to connect to pandas or PySpark after reading data.

Python cons:

- This version is sequential.
- Adding safe parallelism requires choosing between threads, processes, async IO, or a framework.
- Packaging the script for production usually needs dependency and runtime management.

### Go Version

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

func main() {
	paths, _ := filepath.Glob("landing/*.jsonl")

	var wg sync.WaitGroup
	for _, path := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			count, err := countLines(path)
			fmt.Println(path, count, err)
		}(path)
	}
	wg.Wait()
}
```

Go pros:

- Concurrency is built into the language with goroutines.
- The program can be compiled into one binary for a worker or container.
- Good fit for IO-heavy ingestion tasks.

Go cons:

- More boilerplate than Python.
- You must think carefully about error handling from concurrent workers.
- For huge distributed processing, Spark is still usually a better tool than hand-written goroutines.

## Example 3: Aggregating Data

Suppose you want total sales by country.

### Python With pandas

```python
import pandas as pd

orders = pd.read_parquet("orders.parquet")
summary = orders.groupby("country")["amount"].sum().reset_index()
summary.to_parquet("sales_by_country.parquet")
```

Python pros:

- Very concise for analytics.
- pandas and PySpark are excellent for tabular transformations.
- Data engineers can move quickly from exploration to a scheduled job.

Python cons:

- pandas runs on one machine, so large data may require Spark, DuckDB, Polars, or a warehouse.
- Runtime errors can appear late if columns are missing or have unexpected types.
- Deployment can become environment-heavy when many native dependencies are involved.

### Go Version

```go
type Order struct {
	Country string
	Amount  float64
}

func salesByCountry(orders []Order) map[string]float64 {
	totals := map[string]float64{}
	for _, order := range orders {
		totals[order.Country] += order.Amount
	}
	return totals
}
```

Go pros:

- Clear types make the expected data shape obvious.
- Good performance for custom in-memory transformations.
- Useful inside services that calculate metrics while streaming records.

Go cons:

- Less expressive than pandas for tabular data.
- You must write more transformation logic yourself.
- For SQL-style analytics, Python with Spark, DuckDB, or a warehouse is usually faster to build.

## When To Choose Go

Choose Go for a data engineering component when:

- it will run as a service, worker, or CLI
- deployment simplicity matters
- the job is IO-heavy or network-heavy
- concurrency is important
- schemas and contracts should be explicit in code
- the component is part of the platform, not just analysis

Good Go project examples:

- API-to-Kafka event collector
- object storage file validator
- schema registry helper CLI
- webhook ingestion service
- metadata crawler
- lightweight data quality worker

## When To Choose Python

Choose Python when:

- the work is exploratory
- pandas, PySpark, Airflow, dbt, or ML libraries are central
- the team needs fast iteration
- the transformation is mostly SQL or DataFrame logic
- notebooks are part of the workflow

Good Python project examples:

- PySpark ETL job
- ML feature engineering pipeline
- notebook-based data profiling
- Airflow DAG orchestration
- pandas or Polars analysis script

## Practical Recommendation

For many data engineering projects, Go and Python work best together:

```text
Go service or CLI
  -> collects, validates, and writes raw data
  -> publishes events or files

Python or Spark job
  -> transforms data
  -> builds analytics tables
  -> prepares features for ML
```

Go gives you operational reliability. Python gives you analytical speed and ecosystem depth. The best choice depends on which part of the data platform you are building.
