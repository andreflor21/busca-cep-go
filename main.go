package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type ViaCEP struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Unidade     string `json:"unidade"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
	Source      string `json:"source"`
}
type BrasilAPICep struct {
	Cep          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
	Service      string `json:"service"`
	Source       string `json:"source"`
}

func main() {
	http.HandleFunc("/", BuscaCEPHandler)
	fmt.Println("Servidor rodando na porta 8080")
	http.ListenAndServe(":8080", nil)
}

func BuscaCEPHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup

	cepBrasilApi := make(chan BrasilAPICep)
	cepViaCep := make(chan ViaCEP)
	errCh := make(chan error)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	wg.Add(2)

	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	cepParam := r.URL.Query().Get("cep")
	if cepParam == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	go BuscaCepViaCep(ctx, cepParam, cepViaCep, errCh, &wg)
	go BuscaCepBrasilApi(ctx, cepParam, cepBrasilApi, errCh, &wg)
	go func() {
		wg.Wait()
		close(cepBrasilApi)
		close(cepViaCep)
		close(errCh)
	}()

	select {
	case result := <-cepBrasilApi:
		resultJSON, _ := json.Marshal(result)
		w.Header().Set("Content-Type", "application/json")
		w.Write(resultJSON)
	case result := <-cepViaCep:
		resultJSON, _ := json.Marshal(result)
		w.Header().Set("Content-Type", "application/json")
		w.Write(resultJSON)
	case err := <-errCh:
		http.Error(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
	case <-ctx.Done():
		http.Error(w, "Timeout", http.StatusRequestTimeout)
	}

	// w.Write([]byte("Hello World"))
}

func BuscaCep(cep string) (*ViaCEP, error) {
	resp, error := http.Get("https://viacep.com.br/ws/" + cep + "/json/")
	if error != nil {
		return nil, error
	}
	defer resp.Body.Close()
	body, error := io.ReadAll(resp.Body)
	if error != nil {
		return nil, error
	}
	var c ViaCEP
	error = json.Unmarshal(body, &c)
	return &c, nil
}

func BuscaCepBrasilApi(ctx context.Context, cep string, ch chan<- BrasilAPICep, errCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	req, err := http.NewRequestWithContext(ctx, "GET", "https://brasilapi.com.br/api/cep/v1/"+cep, nil)
	if err != nil {
		errCh <- fmt.Errorf("erro ao criar a requisição na BrasilAPI: %w", err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		errCh <- fmt.Errorf("erro ao fazer a requisição na BrasilAPI: %w", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errCh <- fmt.Errorf("erro ao ler o corpo da resposta da BrasilAPI: %w", err)
		return
	}
	var result BrasilAPICep
	err = json.Unmarshal(body, &result)
	if err != nil {
		errCh <- fmt.Errorf("erro ao fazer o decode do corpo da resposta da BrasilAPI: %w", err)
		return
	}
	result.Source = "BrasilAPI"
	select {
	case ch <- result:
	case <-ctx.Done():
		return
	}
}

func BuscaCepViaCep(ctx context.Context, cep string, ch chan<- ViaCEP, errCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://viacep.com.br/ws/"+cep+"/json/", nil)
	if err != nil {
		errCh <- fmt.Errorf("erro ao fazer a requisição na ViaCEP: %w", err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		errCh <- fmt.Errorf("erro ao fazer a requisição na ViaCEP: %w", err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errCh <- fmt.Errorf("erro ao ler o corpo da resposta da ViaCEP: %w", err)
		return
	}
	var result ViaCEP
	err = json.Unmarshal(body, &result)
	if err != nil {
		errCh <- fmt.Errorf("erro ao fazer o decode do corpo da resposta da ViaCEP: %w", err)
		return
	}
	result.Source = "ViaCEP"
	select {
	case ch <- result:
	case <-ctx.Done():
		return
	}
}
