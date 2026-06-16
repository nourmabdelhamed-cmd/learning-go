from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import joblib
import mlflow
import mlflow.sklearn
import pandas as pd
from mlflow.models import infer_signature
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import accuracy_score, confusion_matrix, f1_score, precision_score, recall_score
from sklearn.model_selection import train_test_split
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler


TARGET_COLUMN = "unsafe_maneuver"
METADATA_COLUMNS = ["timestamp", "vehicle_id", "trip_id"]
PREDICTION_COLUMNS = [
    "timestamp",
    "vehicle_id",
    "trip_id",
    "unsafe_maneuver_label",
    "unsafe_maneuver_probability",
    "predicted_unsafe_maneuver",
]

FEATURE_COLUMNS = [
    "speed_kph",
    "accel_x_mps2",
    "accel_y_mps2",
    "yaw_rate_dps",
    "steering_angle_deg",
    "brake_pressure",
    "wheel_vibration",
    "tire_pressure_psi",
    "battery_voltage",
    "camera_temp_c",
    "lidar_temp_c",
    "gps_accuracy_m",
    "road_friction",
    "delta_speed_kph",
    "deceleration_mps2",
    "lateral_g",
    "sensor_temp_delta_c",
    "tire_pressure_delta_psi",
    "health_score",
]


def train_model(
    features_path: Path,
    models_dir: Path,
    predictions_path: Path,
    test_size: float = 0.2,
    random_state: int = 42,
    experiment_name: str = "vehicle-telemetry-unsafe-maneuver",
    tracking_uri: str = "sqlite:///mlflow.db",
    run_name: str = "sklearn-logistic-regression",
) -> dict[str, Any]:
    df = pd.read_csv(features_path)
    require_columns(df, FEATURE_COLUMNS + METADATA_COLUMNS + [TARGET_COLUMN])

    y = parse_bool_series(df[TARGET_COLUMN])
    if y.nunique() < 2:
        raise ValueError(f"{TARGET_COLUMN} needs both true and false examples")

    x = df[FEATURE_COLUMNS].astype(float)
    stratify = y if y.value_counts().min() >= 2 else None
    x_train, x_test, y_train, y_test = train_test_split(
        x,
        y,
        test_size=test_size,
        random_state=random_state,
        stratify=stratify,
    )

    mlflow.set_tracking_uri(tracking_uri)
    mlflow.set_experiment(experiment_name)

    with mlflow.start_run(run_name=run_name) as run:
        model = Pipeline(
            [
                ("scaler", StandardScaler()),
                (
                    "classifier",
                    LogisticRegression(
                        class_weight="balanced",
                        max_iter=1000,
                        random_state=random_state,
                    ),
                ),
            ]
        )
        model.fit(x_train, y_train)

        y_pred = pd.Series(model.predict(x_test), index=y_test.index)
        probabilities = model.predict_proba(x)[:, 1]
        full_predictions = probabilities >= 0.5

        metrics = build_metrics(y_train, y_test, y_pred, test_size, random_state)
        metrics["feature_columns"] = FEATURE_COLUMNS
        metrics["model_type"] = "sklearn.pipeline.Pipeline(StandardScaler, LogisticRegression)"
        metrics["mlflow_run_id"] = run.info.run_id
        metrics["mlflow_experiment_name"] = experiment_name
        metrics["mlflow_tracking_uri"] = tracking_uri

        models_dir.mkdir(parents=True, exist_ok=True)
        predictions_path.parent.mkdir(parents=True, exist_ok=True)

        joblib_path = models_dir / "unsafe_maneuver_model.joblib"
        metrics_path = models_dir / "training_metrics.json"
        joblib.dump(model, joblib_path)
        write_json(metrics_path, metrics)
        write_predictions(df, y, probabilities, full_predictions, predictions_path)

        log_mlflow_run(
            model=model,
            x_train=x_train,
            metrics=metrics,
            features_path=features_path,
            predictions_path=predictions_path,
            metrics_path=metrics_path,
            joblib_path=joblib_path,
            test_size=test_size,
            random_state=random_state,
        )

        return metrics


def log_mlflow_run(
    model: Pipeline,
    x_train: pd.DataFrame,
    metrics: dict[str, Any],
    features_path: Path,
    predictions_path: Path,
    metrics_path: Path,
    joblib_path: Path,
    test_size: float,
    random_state: int,
) -> None:
    mlflow.log_params(
        {
            "features_path": str(features_path),
            "model_family": "logistic_regression",
            "scaler": "StandardScaler",
            "class_weight": "balanced",
            "max_iter": 1000,
            "test_size": test_size,
            "random_state": random_state,
            "feature_count": len(FEATURE_COLUMNS),
            "target_column": TARGET_COLUMN,
        }
    )
    mlflow.log_metrics(
        {
            "accuracy": metrics["accuracy"],
            "precision": metrics["precision"],
            "recall": metrics["recall"],
            "f1": metrics["f1"],
            "train_rows": metrics["train_rows"],
            "test_rows": metrics["test_rows"],
            "positive_train": metrics["positive_train"],
            "positive_test": metrics["positive_test"],
        }
    )
    mlflow.log_artifact(str(predictions_path), artifact_path="predictions")
    mlflow.log_artifact(str(metrics_path), artifact_path="reports")
    mlflow.log_artifact(str(joblib_path), artifact_path="models")

    input_example = x_train.head(5)
    signature = infer_signature(input_example, model.predict(input_example))
    mlflow.sklearn.log_model(
        sk_model=model,
        name="model",
        input_example=input_example,
        signature=signature,
        params={
            "model_family": "logistic_regression",
            "feature_count": len(FEATURE_COLUMNS),
        },
    )


def require_columns(df: pd.DataFrame, required_columns: list[str]) -> None:
    missing = [column for column in required_columns if column not in df.columns]
    if missing:
        raise ValueError(f"missing required columns: {', '.join(missing)}")


def parse_bool_series(series: pd.Series) -> pd.Series:
    if pd.api.types.is_bool_dtype(series):
        return series.astype(bool)

    normalized = series.astype(str).str.strip().str.lower()
    mapping = {
        "true": True,
        "1": True,
        "yes": True,
        "false": False,
        "0": False,
        "no": False,
    }
    parsed = normalized.map(mapping)
    if parsed.isna().any():
        bad_values = sorted(series[parsed.isna()].astype(str).unique())
        raise ValueError(f"invalid boolean values in {series.name}: {bad_values}")
    return parsed.astype(bool)


def build_metrics(
    y_train: pd.Series,
    y_test: pd.Series,
    y_pred: pd.Series,
    test_size: float,
    random_state: int,
) -> dict[str, Any]:
    matrix = confusion_matrix(y_test, y_pred, labels=[False, True])
    true_negatives, false_positives, false_negatives, true_positives = matrix.ravel()

    return {
        "train_rows": int(len(y_train)),
        "test_rows": int(len(y_test)),
        "positive_train": int(y_train.sum()),
        "positive_test": int(y_test.sum()),
        "test_size": test_size,
        "random_state": random_state,
        "accuracy": float(accuracy_score(y_test, y_pred)),
        "precision": float(precision_score(y_test, y_pred, zero_division=0)),
        "recall": float(recall_score(y_test, y_pred, zero_division=0)),
        "f1": float(f1_score(y_test, y_pred, zero_division=0)),
        "confusion_matrix": {
            "true_negatives": int(true_negatives),
            "false_positives": int(false_positives),
            "false_negatives": int(false_negatives),
            "true_positives": int(true_positives),
        },
    }


def write_predictions(
    df: pd.DataFrame,
    labels: pd.Series,
    probabilities: Any,
    predictions: Any,
    predictions_path: Path,
) -> None:
    output = pd.DataFrame(
        {
            "timestamp": df["timestamp"],
            "vehicle_id": df["vehicle_id"],
            "trip_id": df["trip_id"],
            "unsafe_maneuver_label": labels.astype(bool),
            "unsafe_maneuver_probability": probabilities,
            "predicted_unsafe_maneuver": predictions,
        }
    )
    output.to_csv(predictions_path, index=False, columns=PREDICTION_COLUMNS)


def write_json(path: Path, value: dict[str, Any]) -> None:
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train an unsafe maneuver model from Go-generated features.")
    parser.add_argument("--features", type=Path, required=True, help="Path to data/gold/vehicle_features.csv")
    parser.add_argument("--models", type=Path, required=True, help="Directory for model and metric artifacts")
    parser.add_argument("--predictions", type=Path, required=True, help="Output prediction CSV path")
    parser.add_argument("--test-size", type=float, default=0.2, help="Holdout fraction for validation")
    parser.add_argument("--random-state", type=int, default=42, help="Deterministic split/model seed")
    parser.add_argument(
        "--experiment",
        default="vehicle-telemetry-unsafe-maneuver",
        help="MLflow experiment name",
    )
    parser.add_argument(
        "--tracking-uri",
        default="sqlite:///mlflow.db",
        help="MLflow tracking URI, for example sqlite:///mlflow.db or http://localhost:5000",
    )
    parser.add_argument("--run-name", default="sklearn-logistic-regression", help="MLflow run name")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    metrics = train_model(
        features_path=args.features,
        models_dir=args.models,
        predictions_path=args.predictions,
        test_size=args.test_size,
        random_state=args.random_state,
        experiment_name=args.experiment,
        tracking_uri=args.tracking_uri,
        run_name=args.run_name,
    )
    print(
        "training complete: "
        f"accuracy={metrics['accuracy']:.3f} "
        f"precision={metrics['precision']:.3f} "
        f"recall={metrics['recall']:.3f} "
        f"f1={metrics['f1']:.3f} "
        f"mlflow_run_id={metrics['mlflow_run_id']}"
    )


if __name__ == "__main__":
    main()
