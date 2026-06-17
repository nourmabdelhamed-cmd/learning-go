from __future__ import annotations

import argparse
from pathlib import Path

import duckdb


def print_rows(columns: list[str], rows: list[tuple[object, ...]]) -> None:
    print(" | ".join(columns))
    for row in rows:
        print(" | ".join(str(value) for value in row))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--lake", default="lake")
    args = parser.parse_args()

    lake = Path(args.lake)
    samples = lake / "metadata" / "samples.parquet"
    sensors = lake / "metadata" / "sample_sensors.parquet"
    sample_data = lake / "metadata" / "sample_data.parquet"
    metadata_records = lake / "bronze" / "metadata_records.parquet"
    raw_assets = lake / "bronze" / "raw_assets.parquet"
    raw_chunks = lake / "bronze" / "raw_asset_chunks"
    lidar = lake / "features" / "lidar_points.parquet"
    for path in [samples, sensors, sample_data, metadata_records, raw_assets, lidar]:
        if not path.exists():
            raise FileNotFoundError(f"required Parquet artifact missing: {path}")
    if not raw_chunks.is_dir() or not list(raw_chunks.glob("*.parquet")):
        raise FileNotFoundError(f"required Parquet dataset missing: {raw_chunks}/*.parquet")

    con = duckdb.connect()
    print("samples_by_scene")
    print_rows(
        ["scene_id", "samples"],
        con.sql(
            f"""
            select scene_id, count(*) as samples
            from read_parquet('{samples.as_posix()}')
            group by scene_id
            order by scene_id
            """
        ).fetchall(),
    )
    print("sensor_channels")
    print_rows(
        ["sensor_channel", "files"],
        con.sql(
            f"""
            select sensor_channel, count(*) as files
            from read_parquet('{sensors.as_posix()}')
            group by sensor_channel
            order by sensor_channel
            """
        ).fetchall(),
    )
    print("all_sample_data_channels")
    print_rows(
        ["sensor_channel", "is_key_frame", "files"],
        con.sql(
            f"""
            select sensor_channel, is_key_frame, count(*) as files
            from read_parquet('{sample_data.as_posix()}')
            group by sensor_channel, is_key_frame
            order by sensor_channel, is_key_frame
            """
        ).fetchall(),
    )
    print("metadata_records")
    print_rows(
        ["table_name", "records"],
        con.sql(
            f"""
            select table_name, count(*) as records
            from read_parquet('{metadata_records.as_posix()}')
            group by table_name
            order by table_name
            """
        ).fetchall(),
    )
    print("raw_asset_coverage")
    print_rows(
        ["dataset_area", "assets", "bytes", "chunks"],
        con.sql(
            f"""
            select dataset_area, count(*) as assets, sum(size_bytes) as bytes, sum(chunk_count) as chunks
            from read_parquet('{raw_assets.as_posix()}')
            group by dataset_area
            order by dataset_area
            """
        ).fetchall(),
    )
    print("raw_asset_chunks")
    print_rows(
        ["chunks", "bytes"],
        con.sql(
            f"""
            select count(*) as chunks, sum(size_bytes) as bytes
            from read_parquet('{(raw_chunks / "*.parquet").as_posix()}')
            """
        ).fetchall(),
    )
    print("lidar_points")
    print_rows(
        ["sample_id", "points"],
        con.sql(
            f"""
            select sample_id, count(*) as points
            from read_parquet('{lidar.as_posix()}')
            group by sample_id
            order by sample_id
            limit 10
            """
        ).fetchall(),
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
