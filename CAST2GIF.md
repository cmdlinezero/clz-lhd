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

## Reference

Quick Aspect Ratio Reference (--line-height 1.4)

| Target Aspect Ratio | Example Grid |    Best For |
|---------------------|--------------|-------------|
| 16:9 (Standard widescreen) | --cols 120 --rows 28 | GitHub READMEs, YouTube/social embeds
| 21:9 (Ultrawide banner)    | --cols 140 --rows 24 | Wide hero banners like the viewer shown
| 4:3 (Compact)              | --cols 100 --rows 32 | Docs sidebars or narrow containers


```bash
agg --theme github-dark --font-size 64 --cols 180 --rows 24 --line-height 1.4 export/md2hugo-demo.cast export/md2hugo-demo.gif
```
