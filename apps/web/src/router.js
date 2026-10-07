import { createRouter, createWebHistory } from "vue-router";
import LoginPage from "./pages/LoginPage.vue";
import HomePage from "./pages/HomePage.vue";
import CanteenPage from "./pages/CanteenPage.vue";
import StaffPage from "./pages/StaffPage.vue";
import AdminPage from "./pages/AdminPage.vue";
import DisplayPage from "./pages/DisplayPage.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: LoginPage },
    { path: "/home", component: HomePage },
    { path: "/canteens/:id", component: CanteenPage },
    { path: "/staff", component: StaffPage },
    { path: "/admin", component: AdminPage },
    { path: "/display/:id", component: DisplayPage },
  ],
});
