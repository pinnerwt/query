import Router from 'preact-router';
import { useAuth, AuthProvider } from './lib/auth';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import RestaurantEdit from './pages/RestaurantEdit';
import MenuEditor from './pages/MenuEditor';
import Orders from './pages/Orders';

function AppRoutes() {
  const { owner, loading } = useAuth();

  if (loading) {
    return <div class="flex items-center justify-center min-h-screen text-gray-500">載入中...</div>;
  }

  if (!owner) {
    return <Login />;
  }

  return (
    <Router>
      <Dashboard path="/app/" />
      <RestaurantEdit path="/app/restaurants/:id" />
      <MenuEditor path="/app/restaurants/:id/menu" />
      <Orders path="/app/restaurants/:id/orders" />
      <Dashboard default />
    </Router>
  );
}

export function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  );
}
