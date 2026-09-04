# Tradução em dois passos

## Intenção

Hoje o Vox pede ao whisper que ouça português e devolva inglês num passo só
(`--translate`). Com o modelo `small`, esse passo único erra: ele parafraseia o
sentido e, em fala rápida, troca palavras. A queixa do usuário é literalmente
"falo uma coisa e ele transcreve outra".

A mudança separa as duas tarefas. O whisper passa a fazer só o que faz bem —
transcrever português literalmente, com o modelo `medium` — e a tradução para
inglês vira um segundo passo, feito offline pelo modelo Argos pt→en rodando em
CTranslate2. Nada sai da máquina.

Custo medido numa amostra real de 20s: 6,7s ponta a ponta, dos quais só 0,35s
são a tradução. Duas otimizações derrubaram isso dos 12,8s iniciais:
`--audio-ctx 768`, que corta ~30% do encoder sem mudar o texto (em 512 o modelo
perde palavras e entra em loop, ficando mais lento que o padrão), e o corte do
arquivo de hotwords de 312 para 65 termos — a lista longa custava 4,4s de
`prompt time` por ditado, mais do que a própria transcrição.

A lista de hotwords é um orçamento, não um depósito: cada termo é pago em tempo
a cada ditado, então só entram siglas e nomes próprios que o modelo erra de
fato. Palavra comum que ele já acerta (`function`, `deploy`, `Linux`) custa o
mesmo e não devolve nada.

## Critério de aceite

Tradutor isolado responde em inglês correto:

```bash
cd ~/vox && echo "Preciso terminar o front-end HTML CSS antes do deploy, depois abro um pull request no GitHub e rodo os testes de integração da API." \
  | .local/venv-translate/bin/python scripts/translate-pt-en.py
```

Esperado:

```
I need to finish the HTML CSS front-end before deploy, then open a pull request on GitHub and run the API integration tests.
```

Pipeline completo, do áudio ao inglês, sem passar pelo `--translate` do whisper:

```bash
cd ~/vox && VOX_WHISPER_TRANSLATE=0 \
  VOX_WHISPER_MODEL=$PWD/.local/models/whisper-medium-q8_0/ggml-medium-q8_0.bin \
  ./vox transcribe --hotwords ~/.config/vox/hotwords.txt /tmp/amostra.wav 2>/dev/null \
  | .local/venv-translate/bin/python scripts/translate-pt-en.py
```

Esperado: uma frase em inglês contendo `HTML`, `CSS`, `GitHub`, `pull request` e `API`.

Falha do tradutor não pode engolir o ditado — se o segundo passo quebrar, o
texto em português é colado mesmo assim:

```bash
cd ~/vox && echo "teste" | .local/venv-translate/bin/python scripts/translate-pt-en.py --self-check
```

Esperado: `ok: fallback preserva o texto de entrada`.

## Fora de escopo

- Trocar o modelo de transcrição por `large-v3` ou `turbo` — ambos medidos,
  ambos piores em tempo, e o `turbo` também em qualidade.
- Manter processo do whisper quente entre ditados para economizar o load time
  (~320 ms). Ganho pequeno perto dos 12s de inferência.
- Corrigir os idiomatismos do Argos ("o negócio" → "Business", imperativo virando
  3ª pessoa). São erros de estilo, não de conteúdo.
- Segundo atalho de teclado para escolher idioma na hora.
