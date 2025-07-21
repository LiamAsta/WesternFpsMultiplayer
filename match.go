package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand/v2"
	"time"

	"github.com/anthdm/hollywood/actor"
)

type Team int

const (
	TeamLawmen Team = iota
	TeamOutlaws
)

type GameMode int

const (
	SearchAndDestroy GameMode = iota
	TeamDeathmatch
)

type RoundPhase int

const (
	PhaseWarmup RoundPhase = iota
	PhaseBuyTime
	PhaseActive
	PhaseEnd
)

type Match struct {
	p1           *actor.PID
	p2           *actor.PID
	gameMode     GameMode
	currentRound int
	maxRounds    int
	roundTime    time.Duration
	buyTime      time.Duration
	phase        RoundPhase

	// Team assignments
	teams map[*actor.PID]Team

	// Round state
	roundStartTime time.Time
	bombPlanted    bool
	bombPlantTime  time.Time
	bombDefused    bool

	// Score tracking
	lawmenScore  int
	outlawsScore int

	// Player states per round
	playersAlive map[*actor.PID]bool
	playerHealth map[*actor.PID]int

	// New: Weapon system
	playerWeapons map[*actor.PID]*PlayerWeapons
	playerMoney   map[*actor.PID]int

	// Explosion tracking
	activeExplosions []Explosion
}

type Explosion struct {
	Position Position
	Radius   float64
	Damage   int
	Time     time.Time
	OwnerPID *actor.PID
}

type ShootData struct {
	Origin    Position
	Direction Position
	Weapon    WeaponType
}

type ExplosionData struct {
	Position Position
	Radius   float64
	Damage   int
}

func NewMatch(p1, p2 *actor.PID) actor.Producer {
	return func() actor.Receiver {
		teams := make(map[*actor.PID]Team)
		teams[p1] = TeamLawmen
		teams[p2] = TeamOutlaws

		playersAlive := make(map[*actor.PID]bool)
		playersAlive[p1] = true
		playersAlive[p2] = true

		playerHealth := make(map[*actor.PID]int)
		playerHealth[p1] = 100
		playerHealth[p2] = 100

		// Initialize weapon systems
		playerWeapons := make(map[*actor.PID]*PlayerWeapons)
		playerWeapons[p1] = NewPlayerWeapons()
		playerWeapons[p2] = NewPlayerWeapons()

		playerMoney := make(map[*actor.PID]int)
		playerMoney[p1] = 800
		playerMoney[p2] = 800

		return &Match{
			p1:            p1,
			p2:            p2,
			gameMode:      SearchAndDestroy,
			currentRound:  1,
			maxRounds:     16,
			roundTime:     time.Minute * 2,
			buyTime:       time.Second * 15,
			phase:         PhaseWarmup,
			teams:         teams,
			playersAlive:  playersAlive,
			playerHealth:  playerHealth,
			playerWeapons: playerWeapons,
			playerMoney:   playerMoney,
		}
	}
}

func (m *Match) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		log.Printf("Western Showdown iniziato tra %s (Lawmen) e %s (Outlaws)",
			m.p1.String(), m.p2.String())

		m.sendToPlayer(c, m.p1, map[string]interface{}{
			"action": "match_joined",
			"team":   "lawmen",
			"mode":   "search_destroy",
		})

		m.sendToPlayer(c, m.p2, map[string]interface{}{
			"action": "match_joined",
			"team":   "outlaws",
			"mode":   "search_destroy",
		})

		go func() {
			time.Sleep(5 * time.Second)
			c.Send(c.PID(), "start_round")
		}()

	case string:
		switch msg {
		case "start_round":
			m.startNewRound(c)
		case "end_buy_time":
			m.endBuyTime(c)
		case "round_timer":
			m.checkRoundTimer(c)
		case "bomb_exploded":
			m.endRound(c, TeamOutlaws, "Bomb exploded")
		}

	case *PlayerAction:
		m.handlePlayerAction(c, msg)
	}
}

func (m *Match) startNewRound(c *actor.Context) {
	m.phase = PhaseBuyTime
	m.roundStartTime = time.Now()
	m.bombPlanted = false
	m.bombDefused = false
	m.activeExplosions = []Explosion{}

	// Reset player states
	for pid := range m.playersAlive {
		m.playersAlive[pid] = true
		m.playerHealth[pid] = 100

		// Give money based on round result
		if m.currentRound > 1 {
			roundReward := GetRoundReward(false, false, false) // Base reward
			m.playerMoney[pid] += roundReward
		}
	}

	roundData := map[string]interface{}{
		"action":        "round_start",
		"round":         m.currentRound,
		"phase":         "buy_time",
		"buy_time":      int(m.buyTime.Seconds()),
		"time_limit":    int(m.roundTime.Seconds()),
		"lawmen_score":  m.lawmenScore,
		"outlaws_score": m.outlawsScore,
		"money":         m.playerMoney,
	}

	m.sendToPlayer(c, m.p1, roundData)
	m.sendToPlayer(c, m.p2, roundData)

	log.Printf("Round %d iniziato - Buy Phase - Lawmen: %d, Outlaws: %d",
		m.currentRound, m.lawmenScore, m.outlawsScore)

	// Buy time timer
	go func() {
		time.Sleep(m.buyTime)
		c.Send(c.PID(), "end_buy_time")
	}()
}

func (m *Match) endBuyTime(c *actor.Context) {
	m.phase = PhaseActive

	roundData := map[string]interface{}{
		"action": "buy_time_end",
		"phase":  "active",
	}

	m.sendToPlayer(c, m.p1, roundData)
	m.sendToPlayer(c, m.p2, roundData)

	// Start round timer
	go func() {
		time.Sleep(m.roundTime)
		c.Send(c.PID(), "round_timer")
	}()
}

func (m *Match) handlePlayerAction(c *actor.Context, action *PlayerAction) {
	var actionData map[string]interface{}
	if err := json.Unmarshal([]byte(action.Data), &actionData); err != nil {
		log.Printf("Errore parsing azione: %v", err)
		return
	}

	switch action.Action {
	case "buy_weapon":
		if m.phase == PhaseBuyTime {
			m.handleBuyWeapon(c, action.From, actionData)
		}
	case "shoot":
		if m.phase == PhaseActive {
			m.handleAdvancedShoot(c, action.From, actionData)
		}
	case "explosion_damage":
		if m.phase == PhaseActive {
			m.handleExplosionDamage(c, action.From, actionData)
		}
	case "plant_bomb":
		if m.phase == PhaseActive {
			m.handleBombPlant(c, action.From, actionData)
		}
	case "defuse_bomb":
		if m.phase == PhaseActive {
			m.handleBombDefuse(c, action.From, actionData)
		}
	case "move":
		m.handleMove(c, action.From, actionData)
	default:
		log.Printf("Azione non gestita: %s", action.Action)
	}
}

func (m *Match) handleBuyWeapon(c *actor.Context, buyer *actor.PID, data map[string]interface{}) {
	weaponTypeFloat, ok := data["weapon_type"].(float64)
	if !ok {
		return
	}

	weaponType := WeaponType(int(weaponTypeFloat))
	weapon := WeaponStats[weaponType]

	if weapon == nil {
		return
	}

	// Check if player has enough money
	if m.playerMoney[buyer] < weapon.Price {
		m.sendToPlayer(c, buyer, map[string]interface{}{
			"action": "buy_failed",
			"reason": "insufficient_funds",
		})
		return
	}

	// Purchase weapon
	m.playerMoney[buyer] -= weapon.Price
	playerWeapons := m.playerWeapons[buyer]

	if weaponType == WeaponRevolver {
		playerWeapons.Secondary = weaponType
		playerWeapons.SecondaryAmmo = weapon.AmmoCapacity
	} else {
		playerWeapons.Primary = weaponType
		playerWeapons.PrimaryAmmo = weapon.AmmoCapacity
	}

	m.sendToPlayer(c, buyer, map[string]interface{}{
		"action":      "buy_success",
		"weapon_type": weaponType,
		"money":       m.playerMoney[buyer],
	})

	log.Printf("Player %s bought weapon type %d for %d", buyer.String(), weaponType, weapon.Price)
}

func (m *Match) handleAdvancedShoot(c *actor.Context, shooter *actor.PID, data map[string]interface{}) {
	if !m.playersAlive[shooter] {
		return
	}

	playerWeapons := m.playerWeapons[shooter]

	// Check if player can shoot
	if !playerWeapons.CanShoot() {
		return
	}

	// Consume ammo
	if !playerWeapons.Shoot() {
		return
	}

	// Get weapon data
	weapon := playerWeapons.GetCurrentWeapon()

	// Parse shoot data
	originX, _ := data["originX"].(float64)
	originY, _ := data["originY"].(float64)
	originZ, _ := data["originZ"].(float64)
	dirX, _ := data["dirX"].(float64)
	dirY, _ := data["dirY"].(float64)
	dirZ, _ := data["dirZ"].(float64)

	shootOrigin := Position{X: originX, Y: originY, Z: originZ}
	shootDirection := Position{X: dirX, Y: dirY, Z: dirZ}

	_ = shootOrigin
	_ = shootDirection
	// Determine target
	var target *actor.PID
	if shooter == m.p1 {
		target = m.p2
	} else {
		target = m.p1
	}

	if !m.playersAlive[target] {
		return
	}

	// Calculate hit probability based on weapon and distance
	// For simplicity, assume some distance (in real game, get from positions)
	distance := 10.0 // Placeholder
	hitChance := calculateHitChance(weapon, distance)

	// Simulate hit
	if rand.Float64() <= hitChance {
		// Calculate damage
		isHeadshot := rand.Float64() < 0.15 // 15% headshot chance
		damage := CalculateDamage(weapon, distance, isHeadshot)

		// Apply damage
		m.playerHealth[target] -= damage

		// Award money for hit
		m.playerMoney[shooter] += GetKillReward(playerWeapons.Current)

		// Send hit confirmation to shooter
		m.sendToPlayer(c, shooter, map[string]interface{}{
			"action":   "hit_confirmed",
			"damage":   damage,
			"headshot": isHeadshot,
		})

		// Send damage to target
		m.sendToPlayer(c, target, map[string]interface{}{
			"action":   "hit",
			"damage":   damage,
			"health":   m.playerHealth[target],
			"headshot": isHeadshot,
		})

		// Check if player died
		if m.playerHealth[target] <= 0 {
			m.playersAlive[target] = false

			m.sendToPlayer(c, target, map[string]interface{}{
				"action": "player_died",
			})

			m.sendToPlayer(c, shooter, map[string]interface{}{
				"action": "enemy_killed",
				"money":  m.playerMoney[shooter],
			})

			m.checkRoundEndConditions(c)
		}
	}

	// Forward shoot action to other player for visual effects
	shootData := map[string]interface{}{
		"action":      "enemy_shoot",
		"originX":     originX,
		"originY":     originY,
		"originZ":     originZ,
		"dirX":        dirX,
		"dirY":        dirY,
		"dirZ":        dirZ,
		"weapon_type": playerWeapons.Current,
	}

	m.sendToPlayer(c, target, shootData)
}

func (m *Match) handleExplosionDamage(c *actor.Context, exploder *actor.PID, data map[string]interface{}) {
	posX, _ := data["x"].(float64)
	posY, _ := data["y"].(float64)
	posZ, _ := data["z"].(float64)
	radius, _ := data["radius"].(float64)
	damage, _ := data["damage"].(float64)

	explosion := Explosion{
		Position: Position{X: posX, Y: posY, Z: posZ},
		Radius:   radius,
		Damage:   int(damage),
		Time:     time.Now(),
		OwnerPID: exploder,
	}

	m.activeExplosions = append(m.activeExplosions, explosion)

	// Check damage to all players
	for pid := range m.playersAlive {
		if !m.playersAlive[pid] {
			continue
		}

		// For simplicity, assume player is within explosion radius
		// In real implementation, check actual distance
		distance := 5.0 // Placeholder

		if distance <= radius {
			// Calculate damage based on distance
			damageMultiplier := math.Max(0.1, 1.0-(distance/radius))
			actualDamage := int(float64(damage) * damageMultiplier)

			m.playerHealth[pid] -= actualDamage

			m.sendToPlayer(c, pid, map[string]interface{}{
				"action": "explosion_damage",
				"damage": actualDamage,
				"health": m.playerHealth[pid],
			})

			if m.playerHealth[pid] <= 0 {
				m.playersAlive[pid] = false
				m.sendToPlayer(c, pid, map[string]interface{}{
					"action": "player_died",
				})

				// Award kill to exploder
				if exploder != pid {
					m.playerMoney[exploder] += GetKillReward(WeaponDynamite)
					m.sendToPlayer(c, exploder, map[string]interface{}{
						"action": "enemy_killed",
						"money":  m.playerMoney[exploder],
					})
				}
			}
		}
	}

	m.checkRoundEndConditions(c)
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (m *Match) handleMove(c *actor.Context, mover *actor.PID, data map[string]interface{}) {
	// Forward movement to other player
	var recipient *actor.PID
	if mover == m.p1 {
		recipient = m.p2
	} else {
		recipient = m.p1
	}

	moveData := data
	moveData["action"] = "enemy_move"
	m.sendToPlayer(c, recipient, moveData)
}

func calculateHitChance(weapon *Weapon, distance float64) float64 {
	// Base hit chance varies by weapon
	baseChance := 0.8

	switch weapon.Type {
	case WeaponRevolver:
		baseChance = 0.75
	case WeaponShotgun:
		baseChance = 0.9 // High close range accuracy
	case WeaponRifle:
		baseChance = 0.85
	case WeaponDynamite:
		baseChance = 1.0 // Always "hits" but may not damage
	}

	// Reduce chance based on distance and weapon range
	if distance > weapon.Range {
		distancePenalty := (distance - weapon.Range) / weapon.Range
		baseChance *= math.Max(0.1, 1.0-distancePenalty)
	}

	// Factor in weapon spread
	spreadPenalty := weapon.Spread / 10.0
	baseChance *= math.Max(0.3, 1.0-spreadPenalty)

	return math.Min(1.0, math.Max(0.1, baseChance))
}

func (m *Match) checkRoundEndConditions(c *actor.Context) {
	lawmenAlive := 0
	outlawsAlive := 0

	for pid, alive := range m.playersAlive {
		if alive {
			if m.teams[pid] == TeamLawmen {
				lawmenAlive++
			} else {
				outlawsAlive++
			}
		}
	}

	if lawmenAlive == 0 {
		m.endRound(c, TeamOutlaws, "All lawmen eliminated")
	} else if outlawsAlive == 0 {
		m.endRound(c, TeamLawmen, "All outlaws eliminated")
	}
}

func (m *Match) endRound(c *actor.Context, winner Team, reason string) {
	m.phase = PhaseEnd

	// Award round money
	roundReward := GetRoundReward(true, m.bombDefused, m.bombPlanted)
	loseReward := GetRoundReward(false, false, false)

	for pid := range m.playerMoney {
		if m.teams[pid] == winner {
			m.playerMoney[pid] += roundReward
		} else {
			m.playerMoney[pid] += loseReward
		}
	}

	// Update score
	if winner == TeamLawmen {
		m.lawmenScore++
	} else {
		m.outlawsScore++
	}

	endData := map[string]interface{}{
		"action":        "round_end",
		"winner":        m.getTeamName(winner),
		"reason":        reason,
		"lawmen_score":  m.lawmenScore,
		"outlaws_score": m.outlawsScore,
		"money":         m.playerMoney,
	}

	m.sendToPlayer(c, m.p1, endData)
	m.sendToPlayer(c, m.p2, endData)

	log.Printf("Round %d terminato - Vincitore: %s (%s)",
		m.currentRound, m.getTeamName(winner), reason)

	if m.lawmenScore >= 9 || m.outlawsScore >= 9 {
		m.endMatch(c)
	} else {
		m.currentRound++
		go func() {
			time.Sleep(5 * time.Second)
			c.Send(c.PID(), "start_round")
		}()
	}
}

// Rest of the methods remain the same as original...
func (m *Match) handleBombPlant(c *actor.Context, planter *actor.PID, data map[string]interface{}) {
	if m.teams[planter] != TeamOutlaws || m.bombPlanted {
		return
	}

	m.bombPlanted = true
	m.bombPlantTime = time.Now()

	plantData := map[string]interface{}{
		"action":     "bomb_planted",
		"planted_by": m.getPlayerName(planter),
	}

	m.sendToPlayer(c, m.p1, plantData)
	m.sendToPlayer(c, m.p2, plantData)

	log.Printf("Bomba piazzata da %s", planter.String())

	go func() {
		time.Sleep(45 * time.Second)
		if m.bombPlanted && !m.bombDefused {
			c.Send(c.PID(), "bomb_exploded")
		}
	}()
}

func (m *Match) handleBombDefuse(c *actor.Context, defuser *actor.PID, data map[string]interface{}) {
	if m.teams[defuser] != TeamLawmen || !m.bombPlanted || m.bombDefused {
		return
	}

	m.bombDefused = true

	defuseData := map[string]interface{}{
		"action":     "bomb_defused",
		"defused_by": m.getPlayerName(defuser),
	}

	m.sendToPlayer(c, m.p1, defuseData)
	m.sendToPlayer(c, m.p2, defuseData)

	log.Printf("Bomba disinnescata da %s", defuser.String())

	m.endRound(c, TeamLawmen, "Bomb defused")
}

func (m *Match) checkRoundTimer(c *actor.Context) {
	if m.phase != PhaseActive {
		return
	}

	if m.bombPlanted && !m.bombDefused {
		m.endRound(c, TeamOutlaws, "Bomb exploded")
	} else {
		m.endRound(c, TeamLawmen, "Time expired")
	}
}

func (m *Match) endMatch(c *actor.Context) {
	var winner Team
	if m.lawmenScore > m.outlawsScore {
		winner = TeamLawmen
	} else {
		winner = TeamOutlaws
	}

	matchEndData := map[string]interface{}{
		"action": "match_end",
		"winner": m.getTeamName(winner),
		"final_score": map[string]int{
			"lawmen":  m.lawmenScore,
			"outlaws": m.outlawsScore,
		},
	}

	m.sendToPlayer(c, m.p1, matchEndData)
	m.sendToPlayer(c, m.p2, matchEndData)

	log.Printf("Match terminato - Vincitore: %s (%d-%d)",
		m.getTeamName(winner), m.lawmenScore, m.outlawsScore)
}

func (m *Match) sendToPlayer(c *actor.Context, player *actor.PID, data map[string]interface{}) {
	jsonData, _ := json.Marshal(data)
	c.Send(player, &PlayerAction{
		From:   c.PID(),
		Action: data["action"].(string),
		Data:   string(jsonData),
	})
}

func (m *Match) getTeamName(team Team) string {
	switch team {
	case TeamLawmen:
		return "lawmen"
	case TeamOutlaws:
		return "outlaws"
	default:
		return "unknown"
	}
}

func (m *Match) getPlayerName(pid *actor.PID) string {
	if m.teams[pid] == TeamLawmen {
		return "Sheriff"
	}
	return "Outlaw"
}
