# sv

Everything you need to build a Svelte project, powered by [`sv`](https://github.com/sveltejs/cli).

## Creating a project

If you're seeing this, you've probably already done this step. Congrats!

```sh
# create a new project
npx sv create my-app
```

To recreate this project with the same configuration:

```sh
# recreate this project
npx sv@0.12.7 create --template minimal --types ts --no-install frontend
```

## Developing

This project uses **bun** as the package manager.

```sh
bun install
bun run dev
```

## Building

```sh
bun run build
bun run preview
```

**Note:** The original scaffolding commands above used `npx sv`. They are kept for reference only.

> To deploy your app, you may need to install an [adapter](https://svelte.dev/docs/kit/adapters) for your target environment.
