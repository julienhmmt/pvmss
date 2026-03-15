<script lang="ts">
	import { base } from '$app/paths';
	import { auth } from '$lib/stores/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Sheet from '$lib/components/ui/sheet';
	import ThemeToggle from './ThemeToggle.svelte';
	import {
		House,
		PlusSquare,
		MagnifyingGlass,
		BookOpen,
		UserCircle,
		GearSix,
		SignOut,
		List,
		CaretDown
	} from 'phosphor-svelte';

	let mobileOpen = $state(false);

	const navLinks = [
		{ href: '/', icon: House, label: 'Home' },
		{ href: '/vm/create', icon: PlusSquare, label: 'Create VM' },
		{ href: '/search', icon: MagnifyingGlass, label: 'Search VM' },
		{ href: '/docs/user', icon: BookOpen, label: 'Documentation' }
	];

	function navigate(url: string) {
		window.location.href = url;
	}
</script>

<nav
	class="bg-background/95 supports-[backdrop-filter]:bg-background/60 fixed top-0 z-50 w-full border-b backdrop-blur"
>
	<div class="mx-auto flex h-14 max-w-screen-2xl items-center px-4 sm:px-6">
		<!-- Brand -->
		<a href="/" class="mr-6 text-lg font-bold tracking-tight">PVMSS</a>

		<!-- Desktop navigation -->
		<div class="hidden items-center gap-1 md:flex">
			{#each navLinks as link}
				<Button variant="ghost" size="sm" href={link.href}>
					<link.icon class="h-4 w-4" />
					{link.label}
				</Button>
			{/each}
		</div>

		<div class="flex-1"></div>

		<!-- Right side -->
		<div class="flex items-center gap-1">
			<!-- Language selector -->
			<div class="hidden items-center sm:flex">
				<Button variant="ghost" size="sm" href="/set-lang?lang=fr" class="px-2 text-xs">
					FR
				</Button>
				<Button variant="ghost" size="sm" href="/set-lang?lang=en" class="px-2 text-xs">
					EN
				</Button>
			</div>

			<Separator orientation="vertical" class="mx-1 hidden h-4 sm:block" />

			<!-- User dropdown -->
			{#if auth.initialized && auth.username}
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button variant="ghost" size="sm" {...props}>
								<UserCircle class="h-4 w-4" />
								<span class="hidden sm:inline">{auth.username}</span>
								<CaretDown class="h-3 w-3 opacity-50" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-48">
						<DropdownMenu.Label class="font-normal">
							<div class="flex flex-col space-y-1">
								<p class="text-sm font-medium leading-none">{auth.username}</p>
								<p class="text-muted-foreground text-xs leading-none">Administrator</p>
							</div>
						</DropdownMenu.Label>
						<DropdownMenu.Separator />
						{#if auth.isAdmin}
							<DropdownMenu.Item onclick={() => navigate(`${base}/`)}>
								<GearSix class="h-4 w-4" />
								Admin
							</DropdownMenu.Item>
							<DropdownMenu.Separator />
						{/if}
						<DropdownMenu.Item onclick={() => navigate('/logout')}>
							<SignOut class="h-4 w-4" />
							Logout
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/if}

			<ThemeToggle />

			<!-- Mobile menu trigger -->
			<div class="md:hidden">
				<Sheet.Root bind:open={mobileOpen}>
					<Sheet.Trigger>
						{#snippet child({ props })}
							<Button variant="ghost" size="icon" {...props}>
								<List class="h-5 w-5" />
								<span class="sr-only">Menu</span>
							</Button>
						{/snippet}
					</Sheet.Trigger>
					<Sheet.Content side="left">
						<Sheet.Header>
							<Sheet.Title>PVMSS</Sheet.Title>
							<Sheet.Description class="sr-only">Navigation menu</Sheet.Description>
						</Sheet.Header>
						<div class="flex flex-col gap-1 py-4">
							{#each navLinks as link}
								<a
									href={link.href}
									class="hover:bg-accent flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium"
									onclick={() => (mobileOpen = false)}
								>
									<link.icon class="h-4 w-4" />
									{link.label}
								</a>
							{/each}

							<Separator class="my-2" />

							<a
								href="/set-lang?lang=fr"
								class="hover:bg-accent flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium"
							>
								FR — Français
							</a>
							<a
								href="/set-lang?lang=en"
								class="hover:bg-accent flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium"
							>
								EN — English
							</a>

							<Separator class="my-2" />

							{#if auth.isAdmin}
								<a
									href="{base}/"
									class="hover:bg-accent flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium"
									onclick={() => (mobileOpen = false)}
								>
									<GearSix class="h-4 w-4" />
									Admin
								</a>
							{/if}
							<a
								href="/logout"
								class="text-destructive hover:bg-accent flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium"
							>
								<SignOut class="h-4 w-4" />
								Logout
							</a>
						</div>
					</Sheet.Content>
				</Sheet.Root>
			</div>
		</div>
	</div>
</nav>
