import Router from 'preact-router';
import { useAuth, AuthProvider } from './lib/auth';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import RestaurantEdit from './pages/RestaurantEdit';
import MenuEditor from './pages/MenuEditor';
import Orders from './pages/Orders';
import Layout from './components/Layout';

function AppRoutes() {
  const { owner, loading } = useAuth();

  if (loading) {
    return (
      <div class="flex items-center justify-center min-h-screen bg-stone-50">
        <div class="animate-pulse text-slate-400 text-sm">Loading...</div>
      </div>
    );
  }

  if (!owner) {
    return <Login />;
  }

  return (
    <Layout>
      <Router>
        <Dashboard path="/app/" />
        <RestaurantEdit path="/app/restaurants/:id" />
        <MenuEditor path="/app/restaurants/:id/menu" />
        <Orders path="/app/restaurants/:id/orders" />
        <Dashboard default />
      </Router>
    </Layout>
  );
}

export function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  );
}
