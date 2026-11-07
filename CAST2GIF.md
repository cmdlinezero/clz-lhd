# Cast to Gif


## Nix Packages

- [x] asciinema
- [x] asciinema-agg


## Create Cast

```
asciinema rec [output].cast
```

## Create Gif

```
agg --theme github-dark --font-size 64 --rows 30 [input].cast [output].gif
```
