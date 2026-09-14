# Parakeet + Argos como motor padrão

## Intenção

O ditado passa a ser transcrito em português pelo NVIDIA Parakeet TDT 0.6B v3 e
traduzido para inglês pelo Argos local, no lugar do `--translate` do whisper
`small`. Para quem usa nada muda — fala em português, cola inglês — mas o inglês
sai melhor e fala longa não é cortada. O whisper sai do Vox: fica um motor só.

Medido em 40 falas reais do FLEURS pt-BR (split `dev`, 5,6–21,7 s, média 12 s),
em série e com processo frio por ditado, como o Vox roda:

| motor | BLEU en | tempo médio | p90 | fala longa de 115 s |
| --- | --- | --- | --- | --- |
| whisper small `--translate` (padrão antigo) | 36,6 | 4,91 s | 6,90 s | BLEU 34,5 · 205/188 palavras |
| Canary-1B-v2 Q4_K_M, tradução direta | 38,0 | 5,11 s | 7,44 s | BLEU 13,3 · **94/188 palavras** |
| Parakeet v3 q8_0 + Argos | **42,2** | **3,53 s** | **4,75 s** | BLEU 44,6 · 191/188 palavras |

WER do Parakeet no português: 5,0%.

Onde o Parakeet perde é no tempo da fala longa. Ele lê o áudio inteiro de uma
vez e o custo cresce mais rápido que a duração, enquanto o whisper corta em
janelas de 30 s. Pipeline completo, mesmo áudio cortado, em série:

| duração | whisper `--translate` | Parakeet + Argos |
| --- | --- | --- |
| 15 s | 5,5 s | **4,0 s** |
| 30 s | **9,6 s** | 10,2 s |
| 60 s | **13,6 s** | 17,9 s |
| 116 s | **31,5 s** | 37,8 s |

A troca vale pela qualidade, que ganha em todas as durações — e mais na fala
longa (BLEU 44,6 contra 34,5). Em velocidade, só ganha abaixo de ~25 s.

O Canary saiu apesar de traduzir num passo só: na fala longa devolveu metade
das palavras, o mesmo corte silencioso que o `-nt` causava.

> Ressalvas: FLEURS é fala lida, limpa e de domínio geral; ditado real é
> espontâneo e cheio de jargão técnico. O BLEU é implementação própria, em
> minúsculas e sem pontuação, contra uma tradução humana independente — serve
> para comparar os motores entre si, não como número absoluto. O Parakeet não
> aceita prompt de vocabulário, então `hotwords.txt` não tem efeito nele.

## Critério de aceite

Build e testes:

```bash
cd ~/vox && make build && go test ./...
```

Esperado: todos os pacotes `ok`.

Modelo pinado confere:

```bash
cd ~/vox && sha256sum .local/models/parakeet-tdt-0.6b-v3/ggml-parakeet-tdt-0.6b-v3-q8_0.bin
```

Esperado: `4d64e9e96c2792186d072fde0034df0ad670cf680a2f53069052ead827fd600e`.

Fala longa sai inteira pelo motor novo (probe de 116 s; receita do áudio em
`specs/2026-09-04-vad-corta-ditado-longo.md`):

```bash
cd ~/vox && ./vox transcribe /tmp/varied.wav 2>/dev/null | grep -oiE '\bfra[sz]e\b' | wc -l
```

Esperado: `20`.

Pipeline completo devolve inglês:

```bash
cd ~/vox && ./vox transcribe /tmp/varied.wav 2>/dev/null \
  | .local/venv-translate/bin/python scripts/translate-pt-en.py
```

Esperado: o texto das 20 frases, em inglês.

Nenhum resto do motor antigo no código nem nos scripts:

```bash
cd ~/vox && grep -rn "WhisperCPP\|VOX_WHISPER\|VOX_ENGINE\|whisper-cli" \
  --include=*.go internal cmd scripts/vox-desktop-toggle.sh scripts/fetch-whisper.sh | wc -l
```

Esperado: `0`.

## Fora de escopo

- Canary-1B-v2: descartado pela fala longa cortada.
- Vulkan no Iris Xe: pede instalar `glslc` e `vulkan-headers`.
- Levar o script de benchmark para o repositório.
- Vocabulário técnico no Parakeet: o modelo não tem prompt.
