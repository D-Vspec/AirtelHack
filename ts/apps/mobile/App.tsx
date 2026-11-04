import { StatusBar } from "expo-status-bar";
import { useEffect, useState } from "react";
import { Button, ScrollView, StyleSheet, Text, View } from "react-native";
import { createApiClient, STRINGS_EN, type HealthResponse } from "@udyam/shared";

// On a physical device use your computer's LAN IP, e.g. EXPO_PUBLIC_API_URL=http://192.168.1.20:8080/api
const api = createApiClient(process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080/api");

export default function App() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pingResult, setPingResult] = useState<string | null>(null);

  useEffect(() => {
    api.health().then(setHealth).catch((e: unknown) => setError(String(e)));
  }, []);

  async function sendPing() {
    try {
      const { id } = await api.ping("hello from mobile");
      setPingResult(`event ${id} published`);
    } catch (e) {
      setPingResult(`ping failed: ${String(e)}`);
    }
  }

  return (
    <ScrollView contentContainerStyle={styles.container}>
      <Text style={styles.title}>{STRINGS_EN.appName}</Text>
      <Text style={styles.sub}>{STRINGS_EN.tagline}</Text>
      <View style={styles.card}>
        <Text style={styles.cardTitle}>Phase 0 status</Text>
        {error && <Text style={styles.bad}>Gateway unreachable: {error}</Text>}
        {!health && !error && <Text>{STRINGS_EN.checking}</Text>}
        {health &&
          Object.entries(health.services).map(([name, state]) => (
            <View key={name} style={styles.row}>
              <Text>{name}</Text>
              <Text style={state === "ok" ? styles.good : styles.bad}>{state}</Text>
            </View>
          ))}
        <Button title="Send Kafka ping" onPress={sendPing} />
        {pingResult && <Text>{pingResult}</Text>}
      </View>
      <StatusBar style="auto" />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { padding: 24, paddingTop: 64, gap: 12 },
  title: { fontSize: 24, fontWeight: "600" },
  sub: { color: "#52525b" },
  card: { borderWidth: 1, borderColor: "#e4e4e7", borderRadius: 8, padding: 16, gap: 8 },
  cardTitle: { fontWeight: "500" },
  row: { flexDirection: "row", justifyContent: "space-between" },
  good: { color: "#15803d" },
  bad: { color: "#b91c1c" },
});
