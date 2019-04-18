package HeadPatrolSkill

import (
	"mind/core/framework/drivers/distance"
	"mind/core/framework/drivers/hexabody"
	//	"mind/core/framework/log"
	"mind/core/framework/skill"
	"time"
)

const (
	HEAD_ALIGN_SPEED  = 1000 // ms to make initial head alignment
	HEAD_SCAN         = 40   // degrees to scan (centered on walking direction; spins if > 180; still if < 10)
	HEAD_SCAN_SPEED   = 40.0 // degress/s (>10 for smooth movement)
	REACTION_INTERVAL = 2000 // ms frequecy of reaction
	REACTION_DISTANCE = 200  // mm distance to react to (range: 100-1500)
)

type HeadPatrolSkill struct {
	skill.Base
	currentScanDirection hexabody.RotationDirection
	isRunning            bool
}

func NewSkill() skill.Interface {
	// Use this method to create a new skill.
	return &HeadPatrolSkill{}
}

/**
*	Checks current head direction relative to walking direction with knowledge of current rotational direction
* 	to decide whether or not to change the direction of head rotation. Regardless of provided HEAD_SCAN value
*	this function only works with values between 10 and 180 degrees. There is no head motion when less than 10
* 	and there is full, continuous rotation with greater than 180.
 */
func changeHeadDirection(headDirection float64, walkingDirection float64, currentScanDirection hexabody.RotationDirection) bool {
	var validHeadScan float64 = HEAD_SCAN
	switch { // normalized to accepted range
	case HEAD_SCAN < 10:
		validHeadScan = 10
	case HEAD_SCAN > 180:
		validHeadScan = 180
	}

	validHeadDirection := headDirection
	switch { //normalize to walkingDirection
	case headDirection < walkingDirection-validHeadScan/2-90:
		validHeadDirection = headDirection + 360
	case headDirection > walkingDirection+validHeadScan/2+90:
		validHeadDirection = headDirection - 360
	}

	var check float64
	var changeDir bool
	switch currentScanDirection {
	case -1:
		check = walkingDirection - validHeadScan/2
		changeDir = validHeadDirection < check
	case 1:
		check = walkingDirection + validHeadScan/2
		changeDir = validHeadDirection > check
	}
	//	log.Info.Println("Head Check: ", check,"|",validHeadDirection," :: ",headDirection)
	return changeDir
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
	walkingDirection := 0.0
	var currentScanDirection hexabody.RotationDirection = 1
	hexabody.MoveHead(walkingDirection, HEAD_ALIGN_SPEED)
	for {
		var checkInterval time.Duration
		if d.isRunning {
			headDirection := hexabody.Direction()
			if HEAD_SCAN <= 180 && changeHeadDirection(headDirection, walkingDirection, currentScanDirection) {
				currentScanDirection = currentScanDirection * -1
			}
			if HEAD_SCAN >= 10 {
				hexabody.RotateHeadContinuously(currentScanDirection, HEAD_SCAN_SPEED)
			}
			dist, _ := distance.Value()
			//		log.Info.Println("Distance in mm: ", dist, " :: ", err)
			if dist < REACTION_DISTANCE {
				hexabody.StopRotatingHeadContinuously()
				time.Sleep(REACTION_INTERVAL * time.Millisecond)
			}
			var headRatio = HEAD_SCAN / HEAD_SCAN_SPEED
			var toleranceInterval time.Duration = 100 * time.Duration(headRatio) //10% of HEAD_SCAN
			switch {                                                             // determine ideal check interval within bounds
			case toleranceInterval > 1000:
				checkInterval = 1000
			case toleranceInterval < 100:
				checkInterval = 100
			default:
				checkInterval = toleranceInterval
			}
		} else {
			hexabody.StopRotatingHeadContinuously()
		}
		time.Sleep(checkInterval * time.Millisecond)
	}
}

func (d *HeadPatrolSkill) OnDisconnect() {
	// Use this method to do something when the remote disconnected.
	hexabody.Relax()
}

func (d *HeadPatrolSkill) OnRecvJSON(data []byte) {
	// Use this method to do something when skill receive json data from remote client.
}

func (d *HeadPatrolSkill) OnRecvString(data string) {
	// Use this method to do something when skill receive string from remote client.
	switch data {
	case "start":
		d.isRunning = true
	case "stop":
		d.isRunning = false
	}
}
