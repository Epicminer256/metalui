package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ncruces/zenity"
	"github.com/pelletier/go-toml"
	"github.com/rivo/tview"
)

type Config struct {
	General GeneralConfig `toml:"General"`
	Misc    MiscConfig    `toml:"Misc"`
}

type GeneralConfig struct {
	Name           string `toml:"Name"`
	Port           int    `toml:"Port"`
	AuthKey        string `toml:"AuthKey"`
	LogChat        bool   `toml:"LogChat"`
	Tags           string `toml:"Tags"`
	Debug          bool   `toml:"Debug"`
	Private        bool   `toml:"Private"`
	MaxCars        int    `toml:"MaxCars"`
	MaxPlayers     int    `toml:"MaxPlayers"`
	Map            string `toml:"Map"`
	Description    string `toml:"Description"`
	ResourceFolder string `toml:"ResourceFolder"`
}

type MiscConfig struct {
	ImScaredOfUpdates     bool `toml:"ImScaredOfUpdates"`
	SendErrorsShowMessage bool `toml:"SendErrorsShowMessage"`
	SendErrors            bool `toml:"SendErrors"`
}

var config Config = Config{
	General: GeneralConfig{
		Name:           "BeamMP Server",
		Port:           30814,
		AuthKey:        "",
		LogChat:        true,
		Tags:           "Freeroam",
		Debug:          false,
		Private:        true,
		MaxCars:        1,
		MaxPlayers:     8,
		Map:            "/levels/gridmap_v2/info.json",
		Description:    "BeamMP Default Description",
		ResourceFolder: "Resources",
	},
	Misc: MiscConfig{
		ImScaredOfUpdates:     false,
		SendErrorsShowMessage: true,
		SendErrors:            true,
	},
}

var beamserverrunning bool = false
var ngrokserverrunning bool = false

var main_menu_list *tview.List = tview.NewList()
var config_menu *tview.List = tview.NewList()
var remove_mod_menu *tview.List = tview.NewList()
var map_menu *tview.List = tview.NewList()

var app *tview.Application = tview.NewApplication()

var grid *tview.Grid = tview.NewGrid()

var textView *tview.TextView = tview.NewTextView().
	SetDynamicColors(true).
	SetRegions(true)

func main() {

	log("Welcome to MetalUI")

	textView.
		SetChangedFunc(func() {
			app.Draw()
			textView.ScrollToEnd()
		})

	if _, err := os.Stat("BeamMP-Server.exe"); err != nil {
		update()
	}

	if _, err := os.Stat("Resources"); err != nil {
		gen_resources()
	}

	if _, err := os.Stat("ServerConfig.toml"); err != nil {
		configFile, err := os.Create("ServerConfig.toml")
		if err != nil {
			log("Could not create config file")
			return
		}
		log("Created New Config")
		save_config(configFile)
		configFile.Close()
	} else {
		read_config()
	}

	main_menu()

	if err := app.SetRoot(grid, true).SetFocus(grid).Run(); err != nil {
		panic(err)
	}
}

func read_config() {
	file, err := os.Open("ServerConfig.toml")
	if err != nil {
		log("Error opening config file")
	}
	toml.NewDecoder(file).Decode(&config)
}

func save_config(configFile *os.File) {
	if configFile == nil {
		var err error
		configFile, err = os.OpenFile("ServerConfig.toml", os.O_WRONLY, 0644)
		if err != nil {
			log("Could not create config file")
			return
		}
	}
	encoder := toml.NewEncoder(configFile)
	err := encoder.Encode(config)
	if err != nil {
		log("Error writing config file." + err.Error())
		return
	}
	log("Wrote Config")
}

func log(text string) {
	fmt.Fprintf(textView, "%s ", text+"\n")
}

func update() {
	go func() {
		log("Updating server...")
		// Get the data
		resp, err := http.Get("https://github.com/BeamMP/BeamMP-Server/releases/latest/download/BeamMP-Server.exe")
		if err != nil {
			log("Failed to update server: Network Error")
		}
		defer resp.Body.Close()

		// Create the file
		out, err := os.Create("BeamMP-Server.exe")
		if err != nil {
			log("Failed to update server: Could not create file")
		}
		defer out.Close()

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log("Failed to update server: Could not write to file")
		}
		log("Server is up-to-date")
	}()
}

func gen_resources() {
	go func() {
		log("Server resource folder not found, making one.")
		if err := os.Mkdir("Resources", os.ModePerm); err != nil {
			log("Could not make Resources folder")
		} else if err := os.Mkdir("Resources/Client", os.ModePerm); err != nil {
			log("Failed making server resources folder, delete it and try again")
		} else if err := os.Mkdir("Resources/Server", os.ModePerm); err != nil {
			log("Failed making server resources folder, delete it and try again")
		} else {
			log("Created server resources folder")
		}
	}()
}

func switch_menu(menu tview.Primitive) {
	grid.
		Clear()
	grid.
		SetSize(1, 0, -1, -1).
		SetBorders(true).
		AddItem(menu, 0, 0, 1, 1, 1, 1, true).
		AddItem(textView, 0, 1, 1, 1, 1, 1, false)
	app.SetRoot(grid, true).SetFocus(grid)
}

func add_mod() {
	file, err := zenity.SelectFile(
		zenity.Filename(""),
		zenity.FileFilter{Name: "Compressed Mod File", Patterns: []string{"*.zip"}, CaseFold: true},
	)
	if err != nil {
		log("Error opening file: " + err.Error())
		return
	}
	log("Importing " + file)
	var filenameDest string = filepath.Base(file)
	var target string = "Resources/Client/" + filenameDest

	if err := os.Rename(file, target); err != nil {
		linkerr := new(os.LinkError)
		if errors.As(err, &linkerr) {
			inputFile, err := os.Open(file)
			if err != nil {
				log("Couldn't open mod file")
				return
			}
			defer inputFile.Close()

			outputFile, err := os.Create(target)
			if err != nil {
				log("Couldn't make mod file")
				return
			}
			defer outputFile.Close()

			_, err = io.Copy(outputFile, inputFile)
			if err != nil {
				log("Couldn't copy mod")
				return
			}

			inputFile.Close()

			err = os.Remove(file)
			if err != nil {
				log("Couldn't remove source file")
				return
			}
		} else {
			log(err.Error())
		}
	}
	log("Imported " + filenameDest)
}
func mod_tags(mod string) []string {
	tags := []string{"Mod"}

	r, err := zip.OpenReader("Resources/Client/" + mod)
	if err != nil {
		log("Found a mod, but couldn't open it!")
		return tags
	}
	defer r.Close()

	levelReg := regexp.MustCompile(`levels\/?.*`)
	carReg := regexp.MustCompile(`vehicles\/?.*`)
	for _, f := range r.File {
		if levelReg.MatchString(f.Name) {
			if !slices.Contains(tags, "Map") {
				tags = append(tags, "Map")
			}
		}
		if carReg.MatchString(f.Name) {
			if !slices.Contains(tags, "Vehicles") {
				tags = append(tags, "Vehicles")
			}
		}
	}
	return tags
}

func getmaps() []string {
	levels := []string{
		"/levels/gridmap_v2/info.json",
		"/levels/johnson_valley/info.json",
		"/levels/automation_test_track/info.json",
		"/levels/east_coast_usa/info.json",
		"/levels/hirochi_raceway/info.json",
		"/levels/italy/info.json",
		"/levels/jungle_rock_island/info.json",
		"/levels/industrial/info.json",
		"/levels/small_island/info.json",
		"/levels/smallgrid/info.json",
		"/levels/utah/info.json",
		"/levels/west_coast_usa/info.json",
		"/levels/driver_training/info.json",
		"/levels/derby/info.json",
	}
	entries, err := os.ReadDir("Resources/Client")
	if err != nil {
		log("Error! Can't browse mods!")
	}

	for _, mod := range entries {

		r, err := zip.OpenReader("Resources/Client/" + mod.Name())
		if err != nil {
			log("Found a mod, but couldn't open it!")
			return levels
		}
		defer r.Close()

		levelReg := regexp.MustCompile(`levels\/?.*\/info.json`)
		for _, f := range r.File {
			level := levelReg.FindString(f.Name)
			if level != "" {
				if !slices.Contains(levels, "/"+level) {
					levels = append(levels, "/"+level)
				}
			}
		}
	}
	return levels
}

func remove_mod() {
	remove_mod_menu.
		Clear().
		AddItem("Back", "Go back", 'b', main_menu)
	entries, err := os.ReadDir("Resources/Client")
	if err != nil {
		log("Error! Can't browse mods!")
	}

	for _, e := range entries {
		if e.Name()[len(e.Name())-4:] == ".zip" {
			remove_mod_menu.AddItem(e.Name(), strings.Join(mod_tags(e.Name()), ", "), 0, func() {
				config.General.Map = "/levels/gridmap_v2/info.json"
				log(e.Name())
			})
		}

	}
	switch_menu(remove_mod_menu)
}

func spawn_prompt(prompt string, isint bool, callback func(string)) {
	inputField := tview.NewInputField().
		SetLabel(prompt).
		SetFieldWidth(40)
	if isint {
		inputField.SetAcceptanceFunc(tview.InputFieldInteger)
	}
	inputField.SetDoneFunc(func(key tcell.Key) {
		log(inputField.GetText())
		callback(inputField.GetText())
	})

	app.SetRoot(inputField, true).SetFocus(inputField)
}

func settings() {

	config_menu.
		Clear().
		AddItem("Back", "Go back", 'b', main_menu).
		AddItem("Auth Key", "Grab one from https://keymaster.beammp.com/login", 'a', func() {
			spawn_prompt("Enter your auth key (Right click or Ctrl+Shift+V to paste)", false, func(authkey string) {
				config.General.AuthKey = authkey
				save_config(nil)
				settings()
			})
		}).
		AddItem("Server Name", "Imports a mod zip file", 'n', func() {
			spawn_prompt("Enter your server name", false, func(name string) {
				log(name)
				config.General.Name = name
				save_config(nil)
				settings()
			})
		}).
		AddItem("Server Description", "Deletes a mod from the server", 'd', func() {
			spawn_prompt("Enter your server description", false, func(desc string) {
				config.General.Description = desc
				save_config(nil)
				settings()
			})
		}).
		AddItem("Max Players", "Sets the map for the server", 'p', func() {
			spawn_prompt("Enter your Max players", true, func(max string) {
				if i, err := strconv.Atoi(max); err == nil {
					config.General.MaxPlayers = i
					save_config(nil)
				} else {
					log("That is not a number")
				}
				settings()
			})
		}).
		AddItem("Max Cars", "Change Server Settings", 'c', func() {
			spawn_prompt("Enter your Max Cars", true, func(max string) {
				if i, err := strconv.Atoi(max); err == nil {
					config.General.MaxCars = i
					save_config(nil)
				} else {
					log("That is not a number")
				}
				settings()
			})
		}).
		AddItem("Set to Private", "Change Server Settings", 'u', func() {
			spawn_prompt("Set to private? (true/false)", false, func(boob string) {
				if boobi, err := strconv.ParseBool(boob); err == nil {
					config.General.Private = boobi
					save_config(nil)
				} else {
					log("That is not a true false statement")
				}
				settings()
			})
		}).
		AddItem("Server Tags", "For the server browser", 't', func() {
			spawn_prompt("Enter your server tags", false, func(desc string) {
				config.General.Tags = desc
				save_config(nil)
				settings()
			})
		}).
		AddItem("Server port", "For the server browser", 'o', func() {
			spawn_prompt("Enter your server port", true, func(max string) {
				if i, err := strconv.Atoi(max); err == nil {
					config.General.Port = i
					save_config(nil)
				} else {
					log("That is not a number")
				}
				settings()
			})
		})
	switch_menu(config_menu)
}

func set_map() {
	map_menu.
		Clear().
		AddItem("Back", "Go back", 'b', main_menu)

	for m := range getmaps() {
		map_menu.AddItem(getmaps()[m], "Change the map to "+getmaps()[m], 0, main_menu)
	}
	switch_menu(map_menu)
}

func run_server() {
	beamserverrunning = true
	main_menu()
}

func run_ngrok() {
	ngrokserverrunning = true
	main_menu()
}
func stop_ngrok() {
	ngrokserverrunning = false
	main_menu()
}
func stop_server() {
	beamserverrunning = false
	main_menu()
}

func main_menu() {
	main_menu_list.Clear()

	// if beamserverrunning {
	// 	main_menu_list.AddItem("Stop Server", "Kills the server", 'r', stop_server)
	// } else {
	// 	main_menu_list.AddItem("Run Server", "Runs the server", 'r', run_server)
	// }

	// if ngrokserverrunning {
	// 	main_menu_list.AddItem("Stop ngrok", "Kills ngrok", 'n', stop_ngrok)
	// } else {
	// 	main_menu_list.AddItem("Run ngrok", "Runs ngrok to bypass port forwarding", 'n', run_ngrok)
	// }

	main_menu_list.
		AddItem("Add Mod", "Imports a mod zip file", 'a', add_mod).
		AddItem("Delete Mod", "Deletes a mod from the server", 'd', remove_mod).
		AddItem("Set Map", "Sets the map for the server", 'm', set_map).
		AddItem("Server Settings", "Change Server Settings", 's', settings).
		AddItem("Update Server", "Updates the server", 'u', update).
		AddItem("Quit", "Select to exit", 'q', quit)
	switch_menu(main_menu_list)
}

func quit() {
	app.Stop()
}
