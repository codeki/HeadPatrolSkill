package HeadPatrolSkill

import (
	"mind/core/framework/drivers/distance"
	"mind/core/framework/drivers/hexabody"
	"mind/core/framework/log"
	"mind/core/framework/skill"
	"time"
	"strconv"
	"encoding/json"
)

const (
	HEAD_ALIGN_SPEED  = 2000 // ms to make initial head alignment
	HEAD_SCAN_SPEED   = 20.0 // degress/s (>10 for smooth movement)
	REACTION_INTERVAL = 2000 // ms frequecy of reaction
	REACTION_DISTANCE = 200  // mm distance to react to (range: 100-1500)
)

type HeadPatrolSkill struct {
	skill.Base
	currentScanRotation hexabody.RotationDirection
	currentWalkDirection float64
	isRunning            bool
//	headScanSpeed		 float64
	headScanRange		 float64
}

func NewSkill() skill.Interface {
	// Use this method to create a new skill.
	return &HeadPatrolSkill{}
}

/**
*	Checks current head direction relative to walking direction with knowledge of current rotational direction
* 	to decide whether or not to change the direction rotation. Regardless of provided d.headScanRange value
*	this function only works with values between 10 and 180 degrees. There is no head motion when less than 10
* 	and there is full, continuous rotation with greater than 180.
 */
func changeHeadRotation(headDirection float64, d *HeadPatrolSkill) bool {
	var validHeadScanRange float64 = d.headScanRange
	switch { // normalized to accepted range
	case d.headScanRange < 10:
		validHeadScanRange = 10
	case d.headScanRange > 180:
		validHeadScanRange = 180
	}

	validHeadDirection := headDirection
	switch { //normalize to d.currentWalkDirection
	case headDirection < d.currentWalkDirection-validHeadScanRange/2-90:
		validHeadDirection = headDirection + 360
	case headDirection > d.currentWalkDirection+validHeadScanRange/2+90:
		validHeadDirection = headDirection - 360
	}

	var check float64
	var changeRotation bool
	switch d.currentScanRotation {
	case -1:
		check = d.currentWalkDirection - validHeadScanRange/2
		changeRotation = validHeadDirection < check
	case 1:
		check = d.currentWalkDirection + validHeadScanRange/2
		changeRotation = validHeadDirection > check
	}
	//	log.Info.Println("Head Check: ", check,"|",validHeadDirection," :: ",headDirection)
	return changeRotation
}

func (d *HeadPatrolSkill) OnStart() {
	hexabody.Start()
	distance.Start()
	hexabody.Stand()
}

func (d *HeadPatrolSkill) OnClose() {
	hexabody.Close()
	distance.Close()
}

func (d *HeadPatrolSkill) OnConnect() {
	d.currentWalkDirection = 0.0
	d.headScanRange = 30.0
	d.isRunning = true
	d.currentScanRotation = 1
	hexabody.MoveHead(d.currentWalkDirection, HEAD_ALIGN_SPEED)
	for {
		var checkInterval time.Duration
		if d.isRunning && d.headScanRange >= 10 {
			hexabody.RotateHeadContinuously(d.currentScanRotation, HEAD_SCAN_SPEED)
			headDirection := hexabody.Direction()
			if d.headScanRange <= 180 && changeHeadRotation(headDirection, d) {
				d.currentScanRotation = d.currentScanRotation * -1
			}
			dist, _ := distance.Value()
			//		log.Info.Println("Distance in mm: ", dist, " :: ", err)
			if dist < REACTION_DISTANCE {
				hexabody.StopRotatingHeadContinuously()
				time.Sleep(REACTION_INTERVAL * time.Millisecond)
			}
			var headRatio = d.headScanRange / HEAD_SCAN_SPEED
			var toleranceInterval time.Duration = 100 * time.Duration(headRatio) //10% of d.headScanRange
			switch {                                                             // determine ideal check interval within bounds
			case toleranceInterval > 1000:
				checkInterval = 1000
			case toleranceInterval < 100:
				checkInterval = 100
			default:
				checkInterval = toleranceInterval
			}
		} else {
			hexabody.MoveHead(d.currentWalkDirection, HEAD_ALIGN_SPEED)
			hexabody.StopRotatingHeadContinuously()
		}
		time.Sleep(checkInterval * time.Millisecond)
	}
}

func (d *HeadPatrolSkill) OnDisconnect() {
	hexabody.Relax()
}

func (d *HeadPatrolSkill) OnRecvJSON(data []byte) {
	var dat map[string]interface{}
	if err := json.Unmarshal(data, &dat); err != nil {
		panic(err)
	}
//	log.Info.Println("Data byte to string: ",dat)
	run := dat["run"]
	hsr := -1.0
	switch dat["hsr"].(type) {
		case string:
			hsr, _ = strconv.ParseFloat(dat["hsr"].(string), 64)
		default: //nil
	}

	switch run {
	case "start":
		hexabody.MoveHead(d.currentWalkDirection, HEAD_ALIGN_SPEED)
		d.isRunning = true
		log.Info.Println("Starting head scan")
	case "stop":
		d.isRunning = false
		hexabody.MoveHead(d.currentWalkDirection, HEAD_ALIGN_SPEED)
		log.Info.Println("Stopping head scan")
	default: //nil or invalid
	}

	switch {
	case hsr >= 10 && hsr <= 180:
		d.headScanRange = hsr
		log.Info.Println("Changing head scan range to ", hsr)
	case hsr < 10:
		hexabody.MoveHead(d.currentWalkDirection, HEAD_ALIGN_SPEED)
		d.headScanRange = hsr
		log.Info.Println("Staring straight ahead")
	case hsr > 180:
		d.headScanRange = hsr
		log.Info.Println("Scanning in all directions")
	default: //nil or invalid
	}

}


func (d *HeadPatrolSkill) OnRecvString(data string) {
	log.Info.Println("Unrecognized data string received: ", data)
}
