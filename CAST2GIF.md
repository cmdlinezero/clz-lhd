# Cast to Gif


## Nix Packages

- [x] asciinema
- [x] asciinema-agg

```bash
nix-shell -p asciinema asciinema-agg
```

## Create Cast

```bash
asciinema rec [output].cast
```

## Create Gif

```bash
agg --theme github-dark --font-size 64 --rows 30 [input].cast [output].gif
```
