using System.Text.Json;

using Microsoft.Extensions.Configuration;

namespace FluVirus.AntiBruteForce.Mock.Client.Cli;

internal class Program
{
    static async Task Main(string[] args)
    {
        IConfiguration c = new ConfigurationBuilder()
            .AddEnvironmentVariables(prefix: "DOTNET_")
            .Build();

        AppConfiguraion configuraion = c.Get<AppConfiguraion>() ?? throw new Exception("cannot bind client configuration");

        using HttpClient client = new()
        {
            BaseAddress = new Uri(configuraion.ApiServerUrl)
        };

        CancellationTokenSource cts = new();
        Console.CancelKeyPress += (object? sender, ConsoleCancelEventArgs e) => cts.Cancel();

        CancellationToken token = cts.Token;
        while (!token.IsCancellationRequested)
        {
            int randInt = Random.Shared.Next();

            using HttpResponseMessage m = await client.GetAsync($"/echo/{randInt}", cts.Token);
            if (m.IsSuccessStatusCode)
            {
                Stream s = await m.Content.ReadAsStreamAsync(cts.Token);
                Resource? r = await JsonSerializer.DeserializeAsync<Resource>(s, cancellationToken: cts.Token);

                if (r is not null)
                {
                    Console.WriteLine($"By sending {randInt} got {r.Value}");
                }
                else
                {
                    Console.WriteLine("Got troubles during json deserialization");
                }
            }
            else
            {
                Console.WriteLine($"Something gone wrong with request, status code - {m.StatusCode}");
            }

            await Task.Delay(configuraion.Delay ?? 1000);
        }
    }
}
