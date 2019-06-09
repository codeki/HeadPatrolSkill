package HeadPatrolSkill

import (
	"mind/core/framework/drivers/accelerometer"
	"mind/core/framework/drivers/distance"
	"mind/core/framework/drivers/hexabody"
	"mind/core/framework/log"
	"mind/core/framework/skill"
	"time"
	"math"
	"strconv"
	"encoding/json"
)

const (
	APPROACH_SPEED	  = 0.2  // 0.1-1.2cm/s approach to target
	HEAD_ALIGN_SPEED  = 2000 // ms to make initial head alignment
	HEAD_SCAN_SPEED   = 20.0 // degress/s (>10 for smooth movement)
	REACTION_INTERVAL = 2000 // ms frequecy of reaction
	REACTION_DISTANCE = 250  // mm distance to react to (range: 100-1500)
)

type HeadPatrolSkill struct {
	skill.Base
	currentScanRotation hexabody.RotationDirection
	currentWalkDirection float64
	isRunning bool
//	headScanSpeed float64
	headScanRange float64
	powerAx float64
	powerAy float64
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

func approachTarget(targetDistance float64, d *HeadPatrolSkill) {
	hexabody.WalkContinuously(hexabody.Direction(), APPROACH_SPEED)
	time.Sleep(REACTION_INTERVAL * time.Millisecond/2)
	hexabody.StopWalkingContinuously()
	d.currentWalkDirection = hexabody.Direction() + 180.0
	hexabody.WalkContinuously(d.currentWalkDirection, APPROACH_SPEED * 2)
	time.Sleep(REACTION_INTERVAL * time.Millisecond/2)
	hexabody.StopWalkingContinuously()				
}

func powerWave(d *HeadPatrolSkill) {
	hexabody.Stand()
	calibrateTilt(d)
	hexabody.RelaxLegs()
	
	// Same as Toe Position (A,R,H) in simulator, except A ranges -90 to 90	
	var nlp = hexabody.NewLegPositions()
	for i := 0; i < 6; i++ {
	    nlp.SetLegPosition(i, hexabody.NewLegPosition().SetCoordinates(55, 170, -20))
		legPositionGo(nlp, 400)
	}
	hexabody.RelaxLegs()
	checkTilt(d)
}

func powerBow() {
	hexabody.Stand()

	// Same as Toe Position (A,R,H) in simulator, except A ranges -90 to 90	
	var nlp = hexabody.NewLegPositions()
	nlp.SetLegPosition(0, hexabody.NewLegPosition().SetCoordinates(55, 170, -20))
	nlp.SetLegPosition(1, hexabody.NewLegPosition().SetCoordinates(-55, 170, -20))
	legPositionGo(nlp, 2000)
	nlp.SetLegPosition(2, hexabody.NewLegPosition().SetCoordinates(0, 120, 80))
	nlp.SetLegPosition(3, hexabody.NewLegPosition().SetCoordinates(-30, 60, 130))
	nlp.SetLegPosition(4, hexabody.NewLegPosition().SetCoordinates(30, 60, 130))
	nlp.SetLegPosition(5, hexabody.NewLegPosition().SetCoordinates(0, 120, 80))
	legPositionGo(nlp, 2000)
}

func legPositionGo (lps hexabody.LegPositions, legSpeed int) {
	// Check and fit positions
	if !lps.IsValid() {
		log.Info.Println("These positions are unreachale, fit it.")
		lps.Fit()
	}
	// Move legs
	err := hexabody.MoveLegs(lps, legSpeed)
	if err != nil {	
		log.Info.Println(err)
	} else {
		log.Info.Println("Movement complete")
	}	
}

func calibrateTilt (d *HeadPatrolSkill) {
	accelerometer.Start()
	_, _, _, ax, ay, _, _ := accelerometer.Value()
	accelerometer.Close()
	d.powerAx = ax
	d.powerAy = ay
}

func checkTilt(d *HeadPatrolSkill) {
	/*
	* Accelerometer returns acceration (fx, fy, fz) and inclination
	* (ax, ay, az). The y-direction is left/right (legs 2,5) and x-direction 
	* is front/back (legs 0/1, 3/4). The z-direction is redundant and
	* opposite to x. Flat is essentially 0,0 (+/- 1). Positive x
	* means the front is raised. Positive y means the left is raised.
	*/
	accelerometer.Start()
	_, _, _, ax, ay, _, err := accelerometer.Value()
	accelerometer.Close()
	log.Info.Println("Tilt Check: ", ax-d.powerAx, ",", ay-d.powerAy, err)

	if math.Abs(ax-d.powerAx) > 1.5 || math.Abs(ay-d.powerAy) > 1.5 {
		getFlat(ax, ay, d)
	} else {
		log.Info.Println("We are flat!")
	}
}

func getFlat(cax float64, cay float64,d *HeadPatrolSkill) {
	// calibrated x and y inclinations
	// forward direction (power button) is 0, increasing counterclockwise to 360

	var flatDir float64
	var atan2 float64
	atan2 = math.Atan2(cay, cax) //arc tangent of y/x, ranges -Pi to Pi
	if atan2 < 0{
		atan2 = atan2 + 2 * math.Pi // set range to 0 to 2Pi
	}
	flatDir = atan2 * 180 / math.Pi	// radians to degree

	log.Info.Println("Walking direction: ", flatDir)
	hexabody.WalkContinuously(flatDir, APPROACH_SPEED)
	time.Sleep(5000 * time.Millisecond)
	hexabody.StopWalkingContinuously()
	powerWave(d)
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
	d.isRunning = false
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
//			log.Info.Println("Distance in mm: ", dist, " :: ", err)
			if dist < REACTION_DISTANCE {
				// react to proximity detection
				hexabody.StopRotatingHeadContinuously()
				approachTarget(dist, d)

				
				// pause before resuming patrol
				time.Sleep(REACTION_INTERVAL * time.Millisecond)
			}
			var headRatio = d.headScanRange / HEAD_SCAN_SPEED
			var toleranceInterval time.Duration = 100 * time.Duration(headRatio) // 10% of d.headScanRange
			switch {  // set check interval within bounds
			case toleranceInterval > 1000:
				checkInterval = 1000
			case toleranceInterval < 100:
				checkInterval = 100
			default:
				checkInterval = toleranceInterval
			}
		} else {
			hexabody.StopRotatingHeadContinuously()
			hexabody.MoveHead(d.currentWalkDirection, HEAD_ALIGN_SPEED)
		}
		time.Sleep(checkInterval * time.Millisecond)
	}
}

func (d *HeadPatrolSkill) OnDisconnect() {
//	hexabody.Relax()
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
		hexabody.Stand()
		hexabody.MoveHead(d.currentWalkDirection, HEAD_ALIGN_SPEED)
		d.isRunning = true
		log.Info.Println("Starting head scan")
	case "stop":
		d.isRunning = false
		hexabody.RelaxHead()
		log.Info.Println("Stopping head scan")
	case "stand":
		d.isRunning = false
		hexabody.RelaxHead()
		hexabody.Stand()
		log.Info.Println("Stopping head scan and standing by")
	case "sit":
		d.isRunning = false
		hexabody.RelaxHead()
		hexabody.RelaxLegs()
		log.Info.Println("Stopping head scan and resting")
	case "powerbow":
		d.isRunning = false
		hexabody.RelaxHead()
		powerBow()
		log.Info.Println("Stopping head scan and ready to receive power")
	case "powerwave":
		d.isRunning = false
		hexabody.RelaxHead()
		powerWave(d)
		log.Info.Println("Stopping head scan, sitting and waving for power")
	default: //nil or invalid
	}

	switch {
	case hsr >= 10 && hsr <= 180:
		d.headScanRange = hsr
		log.Info.Println("Changing head scan range to ", hsr)
	case hsr < 10:
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
