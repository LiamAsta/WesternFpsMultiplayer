package main

import (
	"math"
	"time"
)

type WeaponType int

const (
	WeaponRevolver WeaponType = iota
	WeaponShotgun
	WeaponRifle
	WeaponDynamite
)

type Weapon struct {
	Type         WeaponType
	Damage       int
	Range        float64
	FireRate     time.Duration
	AmmoCapacity int
	ReloadTime   time.Duration
	Spread       float64 // Dispersione colpi
	Price        int     // Per sistema economico
}

var WeaponStats = map[WeaponType]*Weapon{
	WeaponRevolver: {
		Type:         WeaponRevolver,
		Damage:       35,
		Range:        30.0,
		FireRate:     time.Millisecond * 500,
		AmmoCapacity: 6,
		ReloadTime:   time.Second * 2,
		Spread:       2.0,
		Price:        0, // Arma base
	},
	WeaponShotgun: {
		Type:         WeaponShotgun,
		Damage:       80, // Danno alto ma range basso
		Range:        15.0,
		FireRate:     time.Millisecond * 800,
		AmmoCapacity: 2,
		ReloadTime:   time.Second * 3,
		Spread:       8.0,
		Price:        1200,
	},
	WeaponRifle: {
		Type:         WeaponRifle,
		Damage:       60,
		Range:        50.0,
		FireRate:     time.Millisecond * 750,
		AmmoCapacity: 8,
		ReloadTime:   time.Second * 3,
		Spread:       1.0,
		Price:        2700,
	},
	WeaponDynamite: {
		Type:         WeaponDynamite,
		Damage:       150, // Area damage
		Range:        25.0,
		FireRate:     time.Second * 3,
		AmmoCapacity: 1,
		ReloadTime:   time.Second * 4,
		Spread:       0.0,
		Price:        600,
	},
}

type PlayerWeapons struct {
	Primary       WeaponType
	Secondary     WeaponType
	Current       WeaponType
	PrimaryAmmo   int
	SecondaryAmmo int
	Money         int
}

func NewPlayerWeapons() *PlayerWeapons {
	return &PlayerWeapons{
		Primary:       WeaponRevolver,
		Secondary:     WeaponRevolver,
		Current:       WeaponRevolver,
		PrimaryAmmo:   WeaponStats[WeaponRevolver].AmmoCapacity,
		SecondaryAmmo: WeaponStats[WeaponRevolver].AmmoCapacity,
		Money:         800, // Starting money
	}
}

func (pw *PlayerWeapons) GetCurrentWeapon() *Weapon {
	return WeaponStats[pw.Current]
}

func (pw *PlayerWeapons) GetCurrentAmmo() int {
	if pw.Current == pw.Primary {
		return pw.PrimaryAmmo
	}
	return pw.SecondaryAmmo
}

func (pw *PlayerWeapons) CanShoot() bool {
	return pw.GetCurrentAmmo() > 0
}

func (pw *PlayerWeapons) Shoot() bool {
	if !pw.CanShoot() {
		return false
	}

	if pw.Current == pw.Primary {
		pw.PrimaryAmmo--
	} else {
		pw.SecondaryAmmo--
	}
	return true
}

func (pw *PlayerWeapons) Reload() {
	weapon := pw.GetCurrentWeapon()
	if pw.Current == pw.Primary {
		pw.PrimaryAmmo = weapon.AmmoCapacity
	} else {
		pw.SecondaryAmmo = weapon.AmmoCapacity
	}
}

func (pw *PlayerWeapons) SwitchWeapon() {
	if pw.Current == pw.Primary {
		pw.Current = pw.Secondary
	} else {
		pw.Current = pw.Primary
	}
}

func (pw *PlayerWeapons) BuyWeapon(weaponType WeaponType) bool {
	weapon := WeaponStats[weaponType]
	if pw.Money < weapon.Price {
		return false
	}

	pw.Money -= weapon.Price

	// Determina se è primaria o secondaria
	if weaponType == WeaponRevolver {
		pw.Secondary = weaponType
		pw.SecondaryAmmo = weapon.AmmoCapacity
	} else {
		pw.Primary = weaponType
		pw.PrimaryAmmo = weapon.AmmoCapacity
	}

	return true
}

// Calcola danni in base a distanza e tipo arma
func CalculateDamage(weapon *Weapon, distance float64, isHeadshot bool) int {
	baseDamage := weapon.Damage

	// Riduzione danno per distanza
	if distance > weapon.Range {
		damageReduction := (distance - weapon.Range) / weapon.Range
		baseDamage = int(float64(baseDamage) * math.Max(0.3, 1.0-damageReduction))
	}

	// Headshot multiplier (western style!)
	if isHeadshot {
		baseDamage = int(float64(baseDamage) * 2.5)
	}

	// Shotgun special: multiple pellets
	if weapon.Type == WeaponShotgun {
		// Simula pellets multipli
		pelletHits := 3 + int(math.Max(0, 5-distance/3)) // Più vicino = più pellets
		baseDamage = int(float64(baseDamage) * float64(pelletHits) / 8.0)
	}

	return baseDamage
}

// Sistema economico - reward per azioni
func GetKillReward(weapon WeaponType) int {
	switch weapon {
	case WeaponRevolver:
		return 300
	case WeaponShotgun:
		return 900
	case WeaponRifle:
		return 300
	case WeaponDynamite:
		return 500
	default:
		return 300
	}
}

func GetRoundReward(won bool, bombDefused bool, bombPlanted bool) int {
	reward := 1400 // Base reward

	if won {
		reward += 3250
	} else {
		reward += 1400 // Consolation
	}

	if bombDefused {
		reward += 250
	}
	if bombPlanted {
		reward += 300
	}

	return reward
}
