package nuscenes

type Scene struct {
	Token            string `json:"token"`
	LogToken         string `json:"log_token"`
	NumberOfSamples  int    `json:"nbr_samples"`
	FirstSampleToken string `json:"first_sample_token"`
	LastSampleToken  string `json:"last_sample_token"`
	Name             string `json:"name"`
	Description      string `json:"description"`
}

type Sample struct {
	Token      string `json:"token"`
	Timestamp  int64  `json:"timestamp"`
	Previous   string `json:"prev"`
	Next       string `json:"next"`
	SceneToken string `json:"scene_token"`
}

type SampleData struct {
	Token                 string `json:"token"`
	SampleToken           string `json:"sample_token"`
	EgoPoseToken          string `json:"ego_pose_token"`
	CalibratedSensorToken string `json:"calibrated_sensor_token"`
	Timestamp             int64  `json:"timestamp"`
	FileFormat            string `json:"fileformat"`
	IsKeyFrame            bool   `json:"is_key_frame"`
	Height                int    `json:"height"`
	Width                 int    `json:"width"`
	Filename              string `json:"filename"`
	Previous              string `json:"prev"`
	Next                  string `json:"next"`
}

type Sensor struct {
	Token    string `json:"token"`
	Channel  string `json:"channel"`
	Modality string `json:"modality"`
}

type CalibratedSensor struct {
	Token           string      `json:"token"`
	SensorToken     string      `json:"sensor_token"`
	Translation     []float64   `json:"translation"`
	Rotation        []float64   `json:"rotation"`
	CameraIntrinsic [][]float64 `json:"camera_intrinsic"`
}

type EgoPose struct {
	Token       string    `json:"token"`
	Timestamp   int64     `json:"timestamp"`
	Translation []float64 `json:"translation"`
	Rotation    []float64 `json:"rotation"`
}

type Annotation struct {
	Token           string    `json:"token"`
	SampleToken     string    `json:"sample_token"`
	InstanceToken   string    `json:"instance_token"`
	VisibilityToken string    `json:"visibility_token"`
	AttributeTokens []string  `json:"attribute_tokens"`
	Translation     []float64 `json:"translation"`
	Size            []float64 `json:"size"`
	Rotation        []float64 `json:"rotation"`
	Previous        string    `json:"prev"`
	Next            string    `json:"next"`
	NumLiDARPoints  int       `json:"num_lidar_pts"`
	NumRadarPoints  int       `json:"num_radar_pts"`
	CategoryName    string    `json:"category_name"`
}

type Map struct {
	Token     string   `json:"token"`
	LogTokens []string `json:"log_tokens"`
	Category  string   `json:"category"`
	Filename  string   `json:"filename"`
}

type Dataset struct {
	Scenes            []Scene
	Samples           []Sample
	SampleData        []SampleData
	Sensors           []Sensor
	CalibratedSensors []CalibratedSensor
	EgoPoses          []EgoPose
	Annotations       []Annotation
	Maps              []Map

	scenesByToken       map[string]Scene
	samplesByToken      map[string]Sample
	sensorsByToken      map[string]Sensor
	calibrationsByToken map[string]CalibratedSensor
	egoPosesByToken     map[string]EgoPose
	sampleDataBySample  map[string][]SampleData
}

type SensorFile struct {
	Token                 string
	SampleID              string
	SceneID               string
	EgoPoseID             string
	CalibratedSensorToken string
	SensorChannel         string
	SensorModality        string
	Timestamp             int64
	FileFormat            string
	Width                 int
	Height                int
	Filename              string
	Path                  string
	URI                   string
	test                  bool
}

type SampleBundle struct {
	SampleID      string
	SceneID       string
	SceneToken    string
	Timestamp     int64
	LiDAR         SensorFile
	Cameras       map[string]SensorFile
	EgoPoseID     string
	CalibrationID string
}

type SceneRow struct {
	SceneID         string `parquet:"scene_id"`
	SceneToken      string `parquet:"scene_token"`
	LogToken        string `parquet:"log_token"`
	NumberOfSamples int    `parquet:"number_of_samples"`
	Description     string `parquet:"description"`
}

type SampleRow struct {
	SampleID  string `parquet:"sample_id"`
	SceneID   string `parquet:"scene_id"`
	Timestamp int64  `parquet:"timestamp"`
}

type SampleSensorRow struct {
	SampleID              string `parquet:"sample_id"`
	SceneID               string `parquet:"scene_id"`
	Timestamp             int64  `parquet:"timestamp"`
	SensorChannel         string `parquet:"sensor_channel"`
	SensorModality        string `parquet:"sensor_modality"`
	FileFormat            string `parquet:"file_format"`
	Width                 int    `parquet:"width"`
	Height                int    `parquet:"height"`
	Filename              string `parquet:"filename"`
	Path                  string `parquet:"path"`
	URI                   string `parquet:"uri"`
	EgoPoseID             string `parquet:"ego_pose_id"`
	CalibratedSensorToken string `parquet:"calibrated_sensor_token"`
	CalibrationID         string `parquet:"calibration_id"`
}

type SampleDataRow struct {
	SampleDataID          string `parquet:"sample_data_id"`
	SampleID              string `parquet:"sample_id"`
	SceneID               string `parquet:"scene_id"`
	Timestamp             int64  `parquet:"timestamp"`
	SensorChannel         string `parquet:"sensor_channel"`
	SensorModality        string `parquet:"sensor_modality"`
	FileFormat            string `parquet:"file_format"`
	IsKeyFrame            bool   `parquet:"is_key_frame"`
	Width                 int    `parquet:"width"`
	Height                int    `parquet:"height"`
	Filename              string `parquet:"filename"`
	Path                  string `parquet:"path"`
	URI                   string `parquet:"uri"`
	SizeBytes             int64  `parquet:"size_bytes"`
	EgoPoseID             string `parquet:"ego_pose_id"`
	CalibratedSensorToken string `parquet:"calibrated_sensor_token"`
	SensorToken           string `parquet:"sensor_token"`
	Previous              string `parquet:"previous"`
	Next                  string `parquet:"next"`
}

type EgoPoseRow struct {
	EgoPoseID       string `parquet:"ego_pose_id"`
	Timestamp       int64  `parquet:"timestamp"`
	TranslationJSON string `parquet:"translation_json"`
	RotationJSON    string `parquet:"rotation_json"`
}

type CalibrationRow struct {
	CalibrationID       string `parquet:"calibration_id"`
	CalibratedSensorID  string `parquet:"calibrated_sensor_id"`
	SensorToken         string `parquet:"sensor_token"`
	SensorChannel       string `parquet:"sensor_channel"`
	SensorModality      string `parquet:"sensor_modality"`
	TranslationJSON     string `parquet:"translation_json"`
	RotationJSON        string `parquet:"rotation_json"`
	CameraIntrinsicJSON string `parquet:"camera_intrinsic_json"`
}

type AnnotationRow struct {
	AnnotationID   string `parquet:"annotation_id"`
	SampleID       string `parquet:"sample_id"`
	InstanceToken  string `parquet:"instance_token"`
	NumLiDARPoints int    `parquet:"num_lidar_points"`
	NumRadarPoints int    `parquet:"num_radar_points"`
	SizeJSON       string `parquet:"size_json"`
}

type MapRow struct {
	MapID       string `parquet:"map_id"`
	Category    string `parquet:"category"`
	Filename    string `parquet:"filename"`
	Path        string `parquet:"path"`
	Source      string `parquet:"source"`
	LogTokens   string `parquet:"log_tokens"`
	Description string `parquet:"description"`
}

type CANBusRow struct {
	SceneID            string  `parquet:"scene_id"`
	SampleID           string  `parquet:"sample_id"`
	Timestamp          int64   `parquet:"timestamp"`
	NearestCANTimeUS   int64   `parquet:"nearest_can_time_us"`
	OdomSpeed          float64 `parquet:"odom_speed"`
	SteeringAngle      float64 `parquet:"steering_angle"`
	LongitudinalAccel  float64 `parquet:"longitudinal_accel"`
	TransversalAccel   float64 `parquet:"transversal_accel"`
	YawRate            float64 `parquet:"yaw_rate"`
	NearestIMUTimeUS   int64   `parquet:"nearest_imu_time_us"`
	NearestSteerTimeUS int64   `parquet:"nearest_steer_time_us"`
}

var RequiredCameraChannels = []string{
	"CAM_FRONT",
	"CAM_FRONT_LEFT",
	"CAM_FRONT_RIGHT",
	"CAM_BACK",
	"CAM_BACK_LEFT",
	"CAM_BACK_RIGHT",
}
