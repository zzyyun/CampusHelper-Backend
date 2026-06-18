package snowflake

import (
	"sync"
	"time"
)

const (
	// 时间起始标记点，作为基准时间点（2020-01-01 00:00:00 UTC）
	baseTime = int64(1577836800000)

	// 各部分位数
	timestampBits  = 41 // 时间戳占用41位
	datacenterBits  = 5  // 数据中心占用5位
	machineBits     = 5  // 机器标识占用5位
	sequenceBits    = 12 // 序列号占用12位

	// 各部分最大值
	maxDatacenterID = -1 ^ (-1 << datacenterBits)  // 最大数据中心ID
	maxMachineID    = -1 ^ (-1 << machineBits)     // 最大机器ID
	maxSequence     = -1 ^ (-1 << sequenceBits)    // 最大序列号

	// 各部分左移位数
	timestampShift  = sequenceBits + datacenterBits + machineBits // 时间戳左移位数
	datacenterShift = sequenceBits + machineBits                   // 数据中心左移位数
	machineShift    = sequenceBits                                  // 机器标识左移位数
)

// Snowflake 雪花算法结构体
type Snowflake struct {
	mu           sync.Mutex
	datacenterID int64 // 数据中心ID
	machineID    int64 // 机器ID
	sequence     int64 // 序列号
	lastTime     int64 // 上次生成ID的时间戳（毫秒）
}

// NewSnowflake 创建雪花算法实例
// datacenterID: 数据中心ID (0-31)
// machineID: 机器ID (0-31)
func NewSnowflake(datacenterID, machineID int64) (*Snowflake, error) {
	if datacenterID < 0 || datacenterID > maxDatacenterID {
		return nil, ErrInvalidDatacenterID
	}
	if machineID < 0 || machineID > maxMachineID {
		return nil, ErrInvalidMachineID
	}

	return &Snowflake{
		datacenterID: datacenterID,
		machineID:    machineID,
		sequence:     0,
		lastTime:     0,
	}, nil
}

// NextID 生成下一个唯一ID
func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli() - baseTime

	// 时钟回拨检查
	if now < s.lastTime {
		return 0, ErrClockMovedBackwards
	}

	// 同一毫秒内，序列号递增
	if now == s.lastTime {
		s.sequence = (s.sequence + 1) & maxSequence
		// 序列号溢出，等待下一毫秒
		if s.sequence == 0 {
			for now <= s.lastTime {
				now = time.Now().UnixMilli() - baseTime
			}
		}
	} else {
		// 不同毫秒，序列号重置
		s.sequence = 0
	}

	s.lastTime = now

	// 组装64位ID
	id := (now << timestampShift) |
		(s.datacenterID << datacenterShift) |
		(s.machineID << machineShift) |
		s.sequence

	return id, nil
}

// 全局雪花算法实例（数据中心ID和机器ID都设为1，可根据需要调整）
var defaultSnowflake *Snowflake

func init() {
	var err error
	defaultSnowflake, err = NewSnowflake(1, 1)
	if err != nil {
		panic(err.Error())
	}
}

// GenerateID 生成唯一ID（使用默认实例）
func GenerateID() int64 {
	id, err := defaultSnowflake.NextID()
	if err != nil {
		panic(err.Error())
	}
	return id
}

// 错误定义
var (
	ErrInvalidDatacenterID = &SnowflakeError{Message: "无效的数据中心ID"}
	ErrInvalidMachineID    = &SnowflakeError{Message: "无效的机器ID"}
	ErrClockMovedBackwards = &SnowflakeError{Message: "时钟回拨错误"}
)

// SnowflakeError 雪花算法错误
type SnowflakeError struct {
	Message string
}

func (e *SnowflakeError) Error() string {
	return e.Message
}
