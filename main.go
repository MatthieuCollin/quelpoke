package main

import (
	"crypto/sha1"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed index.tmpl.html
var indexTemplateFS embed.FS

type indexTemplateParams struct {
	Name          string
	Version       string
	PokemonID     uint64
	PokemonName   string
	Types         []string
	Stats         map[string]int
	Evolutions    []string
	PokemonSprite string
}

// env : retourne une valeur d’environnement ou défaut
func env(name string, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func main() {
	addr := env("ADDR", "0.0.0.0")
	port := env("PORT", "8080")
	listen := fmt.Sprintf("%s:%s", addr, port)

	log.Printf("🚀 Server running on http://%s", listen)
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatal(err)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "cafard"
	}

	tmpl, err := template.New("index.tmpl.html").Funcs(template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				return ""
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
	}).ParseFS(indexTemplateFS, "index.tmpl.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	pid := pokemonID(name, 151)
	poke, err := fetchPokemon(pid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	params := indexTemplateParams{
		Name:          name,
		Version:       env("VERSION", "cafard-edition"),
		PokemonID:     pid,
		PokemonName:   poke.Name,
		Types:         poke.Types,
		Stats:         poke.Stats,
		Evolutions:    poke.Evolutions,
		PokemonSprite: poke.Sprite,
	}

	if err := tmpl.Execute(w, params); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	log.Printf("✅ Generated page in %s for %s → %s", time.Since(start), name, poke.Name)
}

func pokemonID(name string, m uint64) uint64 {
	h := sha1.New()
	h.Write([]byte(name))
	return binary.BigEndian.Uint64(h.Sum(nil))%m + 1
}

// Structs pour PokeAPI
type pokeAPIResponse struct {
	Name  string `json:"name"`
	Types []struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"types"`
	Stats []struct {
		BaseStat int `json:"base_stat"`
		Stat     struct {
			Name string `json:"name"`
		} `json:"stat"`
	} `json:"stats"`
	Sprites struct {
		Other struct {
			Official struct {
				Front string `json:"front_default"`
			} `json:"official-artwork"`
		} `json:"other"`
	} `json:"sprites"`
	Species struct {
		URL string `json:"url"`
	} `json:"species"`
}

type pokemonData struct {
	Name       string
	Types      []string
	Stats      map[string]int
	Sprite     string
	Evolutions []string
}

func fetchPokemon(id uint64) (pokemonData, error) {
	var result pokemonData

	// Récupère le Pokémon
	resp, err := http.Get(fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%d", id))
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	var poke pokeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&poke); err != nil {
		return result, err
	}

	result.Name = poke.Name
	result.Sprite = poke.Sprites.Other.Official.Front
	result.Stats = make(map[string]int)
	for _, s := range poke.Stats {
		result.Stats[s.Stat.Name] = s.BaseStat
	}
	for _, t := range poke.Types {
		result.Types = append(result.Types, t.Type.Name)
	}

	// Récupère les évolutions
	result.Evolutions = fetchEvolutions(poke.Species.URL)
	return result, nil
}

func fetchEvolutions(speciesURL string) []string {
	resp, err := http.Get(speciesURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var species struct {
		EvolutionChain struct {
			URL string `json:"url"`
		} `json:"evolution_chain"`
	}
	json.NewDecoder(resp.Body).Decode(&species)

	resp2, err := http.Get(species.EvolutionChain.URL)
	if err != nil {
		return nil
	}
	defer resp2.Body.Close()

	var chain struct {
		Chain struct {
			Species struct {
				Name string `json:"name"`
			} `json:"species"`
			EvolvesTo []struct {
				Species struct {
					Name string `json:"name"`
				} `json:"species"`
				EvolvesTo []struct {
					Species struct {
						Name string `json:"name"`
					} `json:"species"`
				} `json:"evolves_to"`
			} `json:"evolves_to"`
		} `json:"chain"`
	}
	json.NewDecoder(resp2.Body).Decode(&chain)

	evols := []string{chain.Chain.Species.Name}
	for _, e := range chain.Chain.EvolvesTo {
		evols = append(evols, e.Species.Name)
		for _, f := range e.EvolvesTo {
			evols = append(evols, f.Species.Name)
		}
	}
	return evols
}
