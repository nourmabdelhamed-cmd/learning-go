package workflow

type LiDARPointRow struct {
	SampleID          string  `parquet:"sample_id"`
	SceneID           string  `parquet:"scene_id"`
	Timestamp         int64   `parquet:"timestamp"`
	LiDARAssetID      string  `parquet:"lidar_asset_id"`
	LiDARRelativePath string  `parquet:"lidar_relative_path"`
	LiDARPath         string  `parquet:"lidar_path"`
	LiDARSHA256       string  `parquet:"lidar_sha256"`
	LiDARSizeBytes    int64   `parquet:"lidar_size_bytes"`
	ParserVersion     string  `parquet:"parser_version"`
	SourcePointCount  int     `parquet:"source_point_count"`
	DecodedPointCount int     `parquet:"decoded_point_count"`
	PointIndex        int64   `parquet:"point_index"`
	X                 float32 `parquet:"x"`
	Y                 float32 `parquet:"y"`
	Z                 float32 `parquet:"z"`
	Intensity         float32 `parquet:"intensity"`
	Ring              float32 `parquet:"ring"`
}

type ManifestRow struct {
	SampleID              string `json:"sample_id"`
	SceneID               string `json:"scene_id"`
	Timestamp             int64  `json:"timestamp"`
	LiDARPath             string `json:"lidar_path"`
	CamFrontPath          string `json:"cam_front_path"`
	CamFrontLeftPath      string `json:"cam_front_left_path"`
	CamFrontRightPath     string `json:"cam_front_right_path"`
	CamBackPath           string `json:"cam_back_path"`
	CamBackLeftPath       string `json:"cam_back_left_path"`
	CamBackRightPath      string `json:"cam_back_right_path"`
	EgoPoseID             string `json:"ego_pose_id"`
	CalibrationID         string `json:"calibration_id"`
	DatasetVersion        string `json:"dataset_version"`
	SchemaVersion         string `json:"schema_version"`
	CalibrationVersion    string `json:"calibration_version"`
	TransformGraphVersion string `json:"transform_graph_version"`
}

type ParquetManifestRow struct {
	SampleID              string `parquet:"sample_id"`
	SceneID               string `parquet:"scene_id"`
	Timestamp             int64  `parquet:"timestamp"`
	DatasetVersion        string `parquet:"dataset_version"`
	SchemaVersion         string `parquet:"schema_version"`
	CalibrationVersion    string `parquet:"calibration_version"`
	TransformGraphVersion string `parquet:"transform_graph_version"`
	EgoPoseID             string `parquet:"ego_pose_id"`
	CalibrationID         string `parquet:"calibration_id"`

	LiDARAssetID      string `parquet:"lidar_asset_id"`
	LiDARRelativePath string `parquet:"lidar_relative_path"`
	LiDARPath         string `parquet:"lidar_path"`
	LiDARURI          string `parquet:"lidar_uri"`
	LiDARSHA256       string `parquet:"lidar_sha256"`
	LiDARSizeBytes    int64  `parquet:"lidar_size_bytes"`
	LiDARMediaType    string `parquet:"lidar_media_type"`
	LiDARChunkCount   int    `parquet:"lidar_chunk_count"`
	LiDARBytes        []byte `parquet:"lidar_bytes"`

	CamFrontAssetID      string `parquet:"cam_front_asset_id"`
	CamFrontRelativePath string `parquet:"cam_front_relative_path"`
	CamFrontPath         string `parquet:"cam_front_path"`
	CamFrontURI          string `parquet:"cam_front_uri"`
	CamFrontSHA256       string `parquet:"cam_front_sha256"`
	CamFrontSizeBytes    int64  `parquet:"cam_front_size_bytes"`
	CamFrontMediaType    string `parquet:"cam_front_media_type"`
	CamFrontChunkCount   int    `parquet:"cam_front_chunk_count"`
	CamFrontBytes        []byte `parquet:"cam_front_bytes"`

	CamFrontLeftAssetID      string `parquet:"cam_front_left_asset_id"`
	CamFrontLeftRelativePath string `parquet:"cam_front_left_relative_path"`
	CamFrontLeftPath         string `parquet:"cam_front_left_path"`
	CamFrontLeftURI          string `parquet:"cam_front_left_uri"`
	CamFrontLeftSHA256       string `parquet:"cam_front_left_sha256"`
	CamFrontLeftSizeBytes    int64  `parquet:"cam_front_left_size_bytes"`
	CamFrontLeftMediaType    string `parquet:"cam_front_left_media_type"`
	CamFrontLeftChunkCount   int    `parquet:"cam_front_left_chunk_count"`
	CamFrontLeftBytes        []byte `parquet:"cam_front_left_bytes"`

	CamFrontRightAssetID      string `parquet:"cam_front_right_asset_id"`
	CamFrontRightRelativePath string `parquet:"cam_front_right_relative_path"`
	CamFrontRightPath         string `parquet:"cam_front_right_path"`
	CamFrontRightURI          string `parquet:"cam_front_right_uri"`
	CamFrontRightSHA256       string `parquet:"cam_front_right_sha256"`
	CamFrontRightSizeBytes    int64  `parquet:"cam_front_right_size_bytes"`
	CamFrontRightMediaType    string `parquet:"cam_front_right_media_type"`
	CamFrontRightChunkCount   int    `parquet:"cam_front_right_chunk_count"`
	CamFrontRightBytes        []byte `parquet:"cam_front_right_bytes"`

	CamBackAssetID      string `parquet:"cam_back_asset_id"`
	CamBackRelativePath string `parquet:"cam_back_relative_path"`
	CamBackPath         string `parquet:"cam_back_path"`
	CamBackURI          string `parquet:"cam_back_uri"`
	CamBackSHA256       string `parquet:"cam_back_sha256"`
	CamBackSizeBytes    int64  `parquet:"cam_back_size_bytes"`
	CamBackMediaType    string `parquet:"cam_back_media_type"`
	CamBackChunkCount   int    `parquet:"cam_back_chunk_count"`
	CamBackBytes        []byte `parquet:"cam_back_bytes"`

	CamBackLeftAssetID      string `parquet:"cam_back_left_asset_id"`
	CamBackLeftRelativePath string `parquet:"cam_back_left_relative_path"`
	CamBackLeftPath         string `parquet:"cam_back_left_path"`
	CamBackLeftURI          string `parquet:"cam_back_left_uri"`
	CamBackLeftSHA256       string `parquet:"cam_back_left_sha256"`
	CamBackLeftSizeBytes    int64  `parquet:"cam_back_left_size_bytes"`
	CamBackLeftMediaType    string `parquet:"cam_back_left_media_type"`
	CamBackLeftChunkCount   int    `parquet:"cam_back_left_chunk_count"`
	CamBackLeftBytes        []byte `parquet:"cam_back_left_bytes"`

	CamBackRightAssetID      string `parquet:"cam_back_right_asset_id"`
	CamBackRightRelativePath string `parquet:"cam_back_right_relative_path"`
	CamBackRightPath         string `parquet:"cam_back_right_path"`
	CamBackRightURI          string `parquet:"cam_back_right_uri"`
	CamBackRightSHA256       string `parquet:"cam_back_right_sha256"`
	CamBackRightSizeBytes    int64  `parquet:"cam_back_right_size_bytes"`
	CamBackRightMediaType    string `parquet:"cam_back_right_media_type"`
	CamBackRightChunkCount   int    `parquet:"cam_back_right_chunk_count"`
	CamBackRightBytes        []byte `parquet:"cam_back_right_bytes"`
}

type ParquetManifestSummary struct {
	Path          string
	Rows          int
	FirstSampleID string
}

type IngestSummary struct {
	Scenes          int   `json:"scenes"`
	Samples         int   `json:"samples"`
	SensorFiles     int   `json:"sensor_files"`
	SampleDataRows  int   `json:"sample_data_rows"`
	MetadataRecords int   `json:"metadata_records"`
	CANRows         int   `json:"can_rows"`
	MapRows         int   `json:"map_rows"`
	LiDARFiles      int   `json:"lidar_files"`
	LiDARPoints     int   `json:"lidar_points"`
	RawAssets       int   `json:"raw_assets"`
	RawAssetChunks  int   `json:"raw_asset_chunks"`
	RawAssetBytes   int64 `json:"raw_asset_bytes"`
	Events          int   `json:"events"`
}
