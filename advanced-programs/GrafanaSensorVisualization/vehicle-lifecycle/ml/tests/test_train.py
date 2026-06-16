from __future__ import annotations

import json
import os
import shutil
import subprocess
from pathlib import Path

import pandas as pd
import pytest
from mlflow.tracking import MlflowClient

from vehicle_ml.train import FEATURE_COLUMNS, PREDICTION_COLUMNS, train_model


def test_train_model_writes_artifacts_and_predictions(tmp_path):
    features_path = tmp_path / "vehicle_features.csv"
    models_dir = tmp_path / "models"
    predictions_path = tmp_path / "predictions.csv"
    tracking_uri = mlflow_tracking_uri(tmp_path)
    make_feature_frame(rows=80).to_csv(features_path, index=False)

    metrics = train_model(
        features_path,
        models_dir,
        predictions_path,
        experiment_name="test-artifacts",
        tracking_uri=tracking_uri,
    )

    assert (models_dir / "unsafe_maneuver_model.joblib").exists()
    assert (models_dir / "training_metrics.json").exists()
    assert predictions_path.exists()

    persisted_metrics = json.loads((models_dir / "training_metrics.json").read_text())
    for key in ["accuracy", "precision", "recall", "f1", "confusion_matrix"]:
        assert key in persisted_metrics
        assert key in metrics

    predictions = pd.read_csv(predictions_path)
    assert list(predictions.columns) == PREDICTION_COLUMNS
    assert len(predictions) == 80
    assert metrics["mlflow_run_id"]
    assert_mlflow_run_logged(tracking_uri, "test-artifacts")


def test_train_model_loads_go_generated_feature_csv(tmp_path):
    if shutil.which("go") is None:
        pytest.skip("Go toolchain is not available")

    project_root = Path(__file__).resolve().parents[2]
    data_dir = tmp_path / "data"
    env = {
        **os.environ,
        "GOCACHE": str(tmp_path / ".gocache"),
    }
    subprocess.run(
        [
            "go",
            "run",
            "./cmd/vehicle-lifecycle",
            "all",
            "-rows",
            "240",
            "-data",
            str(data_dir),
        ],
        cwd=project_root,
        env=env,
        check=True,
    )

    features_path = data_dir / "gold" / "vehicle_features.csv"
    models_dir = tmp_path / "models"
    predictions_path = data_dir / "gold" / "unsafe_maneuver_predictions.csv"
    tracking_uri = mlflow_tracking_uri(tmp_path)

    metrics = train_model(
        features_path,
        models_dir,
        predictions_path,
        experiment_name="test-go-generated",
        tracking_uri=tracking_uri,
    )

    assert metrics["train_rows"] > 0
    assert metrics["test_rows"] > 0
    assert predictions_path.exists()
    assert_mlflow_run_logged(tracking_uri, "test-go-generated")


def test_train_model_requires_go_feature_schema(tmp_path):
    features_path = tmp_path / "bad_features.csv"
    frame = make_feature_frame(rows=20).drop(columns=["speed_kph"])
    frame.to_csv(features_path, index=False)

    with pytest.raises(ValueError, match="missing required columns: speed_kph"):
        train_model(features_path, tmp_path / "models", tmp_path / "predictions.csv")


def mlflow_tracking_uri(tmp_path: Path) -> str:
    return f"sqlite:///{tmp_path / 'mlflow.db'}"


def assert_mlflow_run_logged(tracking_uri: str, experiment_name: str) -> None:
    client = MlflowClient(tracking_uri=tracking_uri)
    experiment = client.get_experiment_by_name(experiment_name)
    assert experiment is not None
    runs = client.search_runs([experiment.experiment_id])
    assert len(runs) == 1
    run = runs[0]
    assert "f1" in run.data.metrics
    assert run.data.params["model_family"] == "logistic_regression"


def make_feature_frame(rows: int) -> pd.DataFrame:
    data = []
    for i in range(rows):
        unsafe = i % 5 == 0
        speed = 72.0 if unsafe else 42.0
        accel_x = -7.0 if unsafe else 0.2
        accel_y = 5.0 if unsafe else 0.1
        yaw = 45.0 if unsafe else 3.0
        steering = 24.0 if unsafe else 2.0
        brake = 0.9 if unsafe else 0.1

        row = {
            "timestamp": f"2026-01-05T08:00:{i:02d}Z",
            "vehicle_id": f"av-{i % 3:03d}",
            "trip_id": "trip-001",
            "speed_kph": speed,
            "accel_x_mps2": accel_x,
            "accel_y_mps2": accel_y,
            "yaw_rate_dps": yaw,
            "steering_angle_deg": steering,
            "brake_pressure": brake,
            "wheel_vibration": 0.2,
            "tire_pressure_psi": 32.0,
            "battery_voltage": 12.5,
            "camera_temp_c": 36.0,
            "lidar_temp_c": 35.0,
            "gps_accuracy_m": 2.0,
            "road_friction": 0.9,
            "delta_speed_kph": -12.0 if unsafe else 0.5,
            "deceleration_mps2": 7.0 if unsafe else 0.0,
            "lateral_g": 0.5 if unsafe else 0.01,
            "sensor_temp_delta_c": 1.0,
            "tire_pressure_delta_psi": 0.0,
            "health_score": 98.0,
            "sensor_anomaly": False,
            "harsh_braking": unsafe,
            "unsafe_maneuver": unsafe,
            "sensor_drift": False,
            "vehicle_health_issue": False,
            "road_condition": "dry",
        }
        data.append(row)

    frame = pd.DataFrame(data)
    return frame[[
        "timestamp",
        "vehicle_id",
        "trip_id",
        *FEATURE_COLUMNS[:13],
        "delta_speed_kph",
        "deceleration_mps2",
        "lateral_g",
        "sensor_temp_delta_c",
        "tire_pressure_delta_psi",
        "health_score",
        "sensor_anomaly",
        "harsh_braking",
        "unsafe_maneuver",
        "sensor_drift",
        "vehicle_health_issue",
        "road_condition",
    ]]
