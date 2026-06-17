CREATE SCHEMA IF NOT EXISTS bronze;
CREATE SCHEMA IF NOT EXISTS metadata;
CREATE SCHEMA IF NOT EXISTS features;
CREATE SCHEMA IF NOT EXISTS events;

CREATE OR REPLACE VIEW bronze.raw_assets AS
SELECT * FROM read_parquet('lake/bronze/raw_assets.parquet');

CREATE OR REPLACE VIEW bronze.raw_asset_chunks AS
SELECT * FROM read_parquet('lake/bronze/raw_asset_chunks/*.parquet');

CREATE OR REPLACE VIEW bronze.metadata_records AS
SELECT * FROM read_parquet('lake/bronze/metadata_records.parquet');

CREATE OR REPLACE VIEW metadata.samples AS
SELECT * FROM read_parquet('lake/metadata/samples.parquet');

CREATE OR REPLACE VIEW metadata.sample_sensors AS
SELECT * FROM read_parquet('lake/metadata/sample_sensors.parquet');

CREATE OR REPLACE VIEW metadata.sample_data AS
SELECT * FROM read_parquet('lake/metadata/sample_data.parquet');

CREATE OR REPLACE VIEW metadata.scenes AS
SELECT * FROM read_parquet('lake/metadata/scenes.parquet');

CREATE OR REPLACE VIEW metadata.calibrations AS
SELECT * FROM read_parquet('lake/metadata/calibrations.parquet');

CREATE OR REPLACE VIEW metadata.ego_poses AS
SELECT * FROM read_parquet('lake/metadata/ego_poses.parquet');

CREATE OR REPLACE VIEW metadata.annotations AS
SELECT * FROM read_parquet('lake/metadata/annotations.parquet');

CREATE OR REPLACE VIEW metadata.maps AS
SELECT * FROM read_parquet('lake/metadata/maps.parquet');

CREATE OR REPLACE VIEW features.lidar_points AS
SELECT * FROM read_parquet('lake/features/lidar_points.parquet');

CREATE OR REPLACE VIEW features.can_bus AS
SELECT * FROM read_parquet('lake/features/can_bus.parquet');

CREATE OR REPLACE VIEW events.events AS
SELECT * FROM read_parquet('lake/events/events.parquet');