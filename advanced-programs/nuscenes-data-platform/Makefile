CONFIG ?= configs/nuscenes-mini.local.json
FIXTURE_CONFIG ?= configs/nuscenes-mini.fixture.json
REPO_ROOT ?= ../..

.PHONY: setup test inspect-demo ingest-demo query-demo manifest-demo parquet-manifest-demo train-loader-demo train-parquet-loader-demo train-bevfusion-demo train-bevfusion-parquet-demo demo ci-demo clean-artifacts

setup:
	go mod download
	uv sync --dev
	mkdir -p data/raw/nuscenes data/raw/maps runs manifests lake
	test -f $(REPO_ROOT)/v1.0-mini.tgz
	test -f $(REPO_ROOT)/can_bus.zip
	test -f $(REPO_ROOT)/nuScenes-map-expansion-v1.3.zip
	if [ ! -d data/raw/nuscenes/v1.0-mini ]; then tar -xzf $(REPO_ROOT)/v1.0-mini.tgz -C data/raw/nuscenes; fi
	if [ ! -d data/raw/can_bus ]; then unzip -q $(REPO_ROOT)/can_bus.zip -d data/raw; fi
	if [ ! -d data/raw/maps/expansion ]; then unzip -q $(REPO_ROOT)/nuScenes-map-expansion-v1.3.zip -d data/raw/maps; fi

test:
	go test ./...
	PYTHONPATH=python uv run pytest

inspect-demo:
	go run ./cmd/inspect-sample -config $(CONFIG)

ingest-demo:
	go run ./cmd/ingest -config $(CONFIG)

query-demo:
	go run ./cmd/query-demo -lake lake

manifest-demo:
	go run ./cmd/build-manifest -config $(CONFIG)

parquet-manifest-demo:
	go run ./cmd/build-parquet-manifest -config $(CONFIG)

train-loader-demo:
	PYTHONPATH=python uv run python -m training_loader.demo --manifest manifests/training_manifest.jsonl

train-parquet-loader-demo:
	PYTHONPATH=python uv run python -m training_loader.demo --parquet-manifest lake/training/training_manifest_v2.parquet

train-bevfusion-demo:
	PYTHONPATH=python uv run python -m training_loader.train_bevfusion --manifest manifests/training_manifest.jsonl --epochs 3 --batch-size 2

train-bevfusion-parquet-demo:
	PYTHONPATH=python uv run python -m training_loader.train_bevfusion --parquet-manifest lake/training/training_manifest_v2.parquet --epochs 3 --batch-size 2

demo: setup inspect-demo ingest-demo query-demo manifest-demo parquet-manifest-demo train-loader-demo train-parquet-loader-demo train-bevfusion-demo train-bevfusion-parquet-demo

ci-demo:
	go run ./cmd/ingest -config $(FIXTURE_CONFIG)
	go run ./cmd/query-demo -lake runs/ci/lake
	go run ./cmd/build-manifest -config $(FIXTURE_CONFIG)
	go run ./cmd/build-parquet-manifest -config $(FIXTURE_CONFIG)
	PYTHONPATH=python uv run python -m training_loader.demo --manifest runs/ci/manifests/training_manifest.jsonl
	PYTHONPATH=python uv run python -m training_loader.demo --parquet-manifest runs/ci/lake/training/training_manifest_v2.parquet

clean-artifacts:
	rm -rf data lake manifests runs .pytest_cache .uv-cache .gocache python/training_loader.egg-info
